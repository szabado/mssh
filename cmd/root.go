package cmd

import (
	"fmt"
	"os"
	"sort"
	"sync"

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
	rootCmd.PersistentFlags().StringVarP(&file, "file", "f", "", "List of hostnames in a file (/dev/stdin for reading from stdin).")
	rootCmd.PersistentFlags().IntVarP(&maxFlight, "maxflight", "m", 50, "Maximum number of concurrent connections.")
	rootCmd.PersistentFlags().IntVarP(&timeout, "timeout", "t", 60, "How many seconds may each individual call take? 0 for no timeout.")
	// TODO: add an alias for global_timeout for backwards compatibility
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
		if file != "" {
			panic("--file is not implemented")
		}

		if collapse {
			panic("--collapse not implemented yet")
		}

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
	hosts, err := parseHostsArg(hostsArg)
	if err != nil {
		panic(err)
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

	// TODO: implement timeouts
	for _, h := range hosts {
		log.WithField("host", h.hostName).Debug("Creating job for host")
		jobs <- &job{
			host:    h,
			command: command,
		}
	}
	close(jobs)
	wg.Wait()

	close(results)
	<-resultsFinished
}

func executor(queue <-chan *job, results chan<- *result, shutdown <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case j, ok := <-queue:
			if !ok {
				return
			}
			logger := log.WithField("host", j.host)
			logger.Debug("Received job from queue")
			results <- handleJob(j)
			logger.Debug("Submitted results for job")
		case <-shutdown:
			// TODO: handle this gracefully so you know what commands have finished
			return
		}
	}
}

func handleJob(j *job) *result {
	logger := log.WithField("host", j.host)
	logger.Debug("Connecting to host")
	h, err := connectToHost(j.host)
	if err != nil {
		return &result{err: err}
	}
	defer h.Close()

	logger.Debug("Establishing new session")
	s, err := h.NewSession()
	if err != nil {
		return &result{err: err}
	}
	defer s.Close()

	logger.WithField("command", j.command).Debug("Running command")
	o, err := s.CombinedOutput(j.command)
	logger.WithField("command", j.command).Debug("Command finished")
	return &result{
		host:   j.host,
		output: o,
		err:    err,
	}
}

func aggregator(results <-chan *result, resultsFinished chan<- struct{}) {
	output := make(map[string]*result)
	hosts := make([]string, 0)

	for result := range results {
		if collapse {
			// TODO
		} else {
			output[result.host.String()] = result
			hosts = append(hosts, result.host.String())
		}
	}

	sort.Strings(hosts)
	for _, h := range hosts {
		fmt.Println(outputBar)
		fmt.Printf("host: %s\n", h)

		r := output[h]
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
