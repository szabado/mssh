package cmd

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	command       string
	hostsArg      string
	file          string
	maxFlight     int
	timeout       int
	globalTimeout int
	collapse      bool
	verbose       bool
	debug         bool
)

type job struct {
	host    *host
	command string
}

type result struct {
	host   *host
	output []byte
	err    error
}

const (
	outputBar = "==================================="
)

func init() {
	rootCmd.PersistentFlags().StringVar(&hostsArg, "hosts", "", "Comma separated list of hostnames to execute on (format [user@]host[:port]). User defaults to the current user. Port defaults to 22.")
	rootCmd.PersistentFlags().StringVarP(&file, "file", "f", "", "List of hostnames in a file (/dev/stdin for reading from stdin). Host names can be separated by commas or whitespace.")
	rootCmd.PersistentFlags().IntVarP(&maxFlight, "maxflight", "m", 50, "Maximum number of concurrent connections.")
	rootCmd.PersistentFlags().IntVarP(&timeout, "timeout", "t", 60, "How many seconds may each individual call take? 0 for no timeout.")
	rootCmd.PersistentFlags().IntVarP(&globalTimeout, "timeout_global", "g", 600, "How many seconds for all calls to take? 0 for no timeout.")
	rootCmd.PersistentFlags().BoolVarP(&collapse, "collapse", "c", false, "Collapse similar output.")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output (INFO level).")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Debug output (DEBUG level).")
}

var rootCmd = &cobra.Command{
	Use:   "mssh [command]",
	Short: "A tool for running multiple commands and ssh jobs in parallel, and easily collecting the results",
	Args:  cobra.ExactArgs(1),
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		log.SetLevel(log.FatalLevel)
		if verbose {
			log.SetLevel(log.InfoLevel)
		}
		if debug {
			log.SetLevel(log.DebugLevel)
		}

		command = args[0]
		return nil
	},
	Run: RunRoot,
}

func RunRoot(cmd *cobra.Command, args []string) {
	if file != "" {
		var err error
		hostsArg, err = loadFileContents(file)
		if err != nil {
			log.WithError(err).Fatal("Could not parse input file")
		}
	}

	hosts, err := parseHostsArg(hostsArg)
	if err != nil {
		log.WithError(err).Fatal("Could not parse hosts")
	}

	// No point in extra goroutines
	if len(hosts) < maxFlight {
		maxFlight = len(hosts)
	}

	jobs := make(chan *job, maxFlight)
	shutdown := make(chan struct{})
	results := make(chan *result, maxFlight)
	resultsFinished := make(chan struct{})

	wg := &sync.WaitGroup{}
	wg.Add(maxFlight)

	go aggregator(results, resultsFinished)
	for i := 0; i < maxFlight; i++ {
		go executor(jobs, results, shutdown, wg)
	}

	done := make(chan struct{})
	// TODO: implement timeouts
	go func() {
		for _, h := range hosts {
			log.WithField("host", h.hostName).Debug("Creating job for host")
			jobs <- &job{
				host:    h,
				command: command,
			}
		}
		close(jobs)

		wg.Wait()
		close(done)
	}()

	select {
	case <-time.After(time.Duration(globalTimeout) * time.Second):
		// Timed out
		close(shutdown)
		wg.Wait()

	case <-done:
		// Do nothing
	}
	close(results)
	<-resultsFinished
}

func executor(queue <-chan *job, results chan<- *result, shutdown <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		// Give the shutdown channel priority
		select {
		case <-shutdown:
			// ignore anything else that might be in the queue, terminate
			log.Debug("Shutting down worker")
			return
		default:
			// Do nothing
		}

		select {
		case j, ok := <-queue:
			if !ok {
				return
			}
			logger := log.WithField("host", j.host)
			logger.Debug("Received job from queue")
			results <- handleJob(j, shutdown)
			logger.Debug("Submitted results for job")
		case <-shutdown:
			log.Debug("Shutting down worker")
			return
		}
	}
}

func handleJob(j *job, shutdown <-chan struct{}) *result {
	done := make(chan *result)

	timeout := time.Duration(timeout) * time.Second

	logger := log.WithField("host", j.host)
	logger.Debug("Connecting to host")

	go func() {
		r := &result{
			host: j.host,
		}
		defer func() {
			done <- r
		}()

		h, err := connectToHost(j.host, timeout)
		if err != nil {
			r.err = err
			return
		}
		defer h.Close()

		logger.Debug("Establishing new session")
		s, err := h.NewSession()
		if err != nil {
			r.err = err
			return
		}
		defer s.Close()

		logger.WithField("command", j.command).Debug("Running command")
		o, err := s.CombinedOutput(j.command)
		logger.WithField("command", j.command).Debug("Command finished")

		r.output = o
		return
	}()

	select {
	case r := <-done:
		return r
	case <-time.After(timeout):
		logger.Info("Command timed out")

		return &result{
			host: j.host,
			err:  errors.New("Command timed out"),
		}
	case <-shutdown:
		// Global timeout has triggered, time to die
		return &result{
			host: j.host,
			err:  errors.New("Global timeout fired, command interrupted"),
		}
	}
}

// Returns a combination of the output from the result
func joinLogs(r *result) string {
	if r.err == nil {
		return string(r.output)
	}
	return fmt.Sprintf("%s%s", r.err, r.output)
}

func aggregator(results <-chan *result, resultsFinished chan<- struct{}) {
	output := make(map[string][]*result)

	for r := range results {
		key := r.host.String()
		if collapse {
			key = joinLogs(r)
		}

		output[key] = append(output[key], r)
	}

	for _, rs := range output {
		fmt.Println(outputBar)
		hosts := ""
		for i := 0; i < len(rs); i++ {
			hosts += rs[i].host.String()
			if i != len(rs)-1 {
				hosts += ", "
			}
		}
		fmt.Printf("host: %s\n", hosts)

		r := rs[0]
		fmt.Print("result: ")
		if r.err != nil {
			fmt.Println("FAILED")
			fmt.Printf("mssh error: %s\n", r.err)
		} else {
			fmt.Println("OK")
		}
		fmt.Printf("command output: %s\n", r.output)
	}

	close(resultsFinished)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
