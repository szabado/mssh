package cli

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/szabado/mssh/ssh"
)

var (
	commandArg       string
	hostsArg         string
	fileArg          string
	maxFlightArg     int
	timeoutArg       int
	globalTimeoutArg int
	collapseArg      bool
	verboseArg       bool
	debugArg         bool
	useGoSSHArg      bool

	hosts []*ssh.Host
)

type job struct {
	host    *ssh.Host
	command string
}

type result struct {
	host   *ssh.Host
	output []byte
	err    error
}

const (
	outputBar = "==================================="
)

func init() {
	rootCmd.PersistentFlags().StringVar(&hostsArg, "hosts", "", "Comma separated list of hostnames to execute on (format [user@]host[:port]). User defaults to the current user. Port defaults to 22.")
	rootCmd.PersistentFlags().StringVarP(&fileArg, "file", "f", "", "List of hostnames in a file (/dev/stdin for reading from stdin). Host names can be separated by commas or whitespace.")
	rootCmd.PersistentFlags().IntVarP(&maxFlightArg, "maxflight", "m", 50, "Maximum number of concurrent connections.")
	rootCmd.PersistentFlags().IntVarP(&timeoutArg, "timeout", "t", 60, "How many seconds may each individual call take? 0 for no timeout.")
	rootCmd.PersistentFlags().IntVarP(&globalTimeoutArg, "timeout_global", "g", 600, "How many seconds for all calls to take? 0 for no timeout.")
	rootCmd.PersistentFlags().BoolVarP(&collapseArg, "collapse", "c", false, "Collapse similar output.")
	rootCmd.PersistentFlags().BoolVarP(&useGoSSHArg, "disable-open-ssh", "o", false, "Disable OpenSSH in favour of the Go SSH library. Disabling causes mssh to ignore ~/.ssh/config; mssh will still talk to ssh-agent to get credentials.")
	rootCmd.PersistentFlags().BoolVarP(&verboseArg, "verbose", "v", false, "Verbose output (INFO level).")
	rootCmd.PersistentFlags().BoolVarP(&debugArg, "debug", "d", false, "Debug output (DEBUG level).")
}

var rootCmd = &cobra.Command{
	Use:   "mssh [command]",
	Short: "A tool for running multiple commands and ssh jobs in parallel, and easily collecting the results",
	Args:  cobra.ExactArgs(1),
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		log.SetLevel(log.FatalLevel)
		if verboseArg {
			log.SetLevel(log.InfoLevel)
		}
		if debugArg {
			log.SetLevel(log.DebugLevel)
		}

		commandArg = args[0]

		if fileArg != "" {
			var err error
			hostsArg, err = loadFileContents(fileArg)
			if err != nil {
				log.WithError(err).Fatal("Could not parse input file")
			}
		}

		var err error
		hosts, err = parseHostsArg(hostsArg)
		if err != nil {
			log.WithError(err).Fatal("Could not parse hosts")
		}
		return nil
	},
	Run: runRoot,
}

func runRoot(cmd *cobra.Command, args []string) {
	maxFlight := maxFlightArg
	// No point in extra goroutines
	if len(hosts) < maxFlight {
		maxFlight = len(hosts)
	}

	jobs := make(chan *job, maxFlight)
	shutdown := make(chan struct{})
	results := make(chan *result, maxFlight)
	resultsFinished := make(chan struct{})

	go aggregator(results, resultsFinished, collapseArg)

	wg := &sync.WaitGroup{}
	wg.Add(maxFlight)
	for i := 0; i < maxFlight; i++ {
		go executor(jobs, results, shutdown, wg, time.Duration(timeoutArg)*time.Second, useGoSSHArg)
	}

	go jobGenerator(jobs, hosts)

	timeoutProxy := make(chan time.Time)
	if globalTimeoutArg != 0 {
		go func() {
			t := <-time.After(time.Duration(globalTimeoutArg) * time.Second)
			timeoutProxy <- t
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-timeoutProxy:
		// Timed out
		close(shutdown)
		<-done
	case <-done:
		// Do nothing
	}
	close(results)
	<-resultsFinished
}

func jobGenerator(jobs chan<- *job, hosts []*ssh.Host) {
	for _, h := range hosts {
		log.WithField("host", h.Hostname).Debug("Creating job for host")
		jobs <- &job{
			host:    h,
			command: commandArg,
		}
	}
	close(jobs)
}

func executor(queue <-chan *job, results chan<- *result, shutdown <-chan struct{}, wg *sync.WaitGroup, timeout time.Duration, useGoSSH bool) {
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
			results <- handleJob(j, shutdown, timeout, useGoSSH)
			logger.Debug("Submitted results for job")
		case <-shutdown:
			log.Debug("Shutting down worker")
			return
		}
	}
}

func handleJob(j *job, shutdown <-chan struct{}, timeout time.Duration, useGoSSH bool) *result {
	done := make(chan *result)

	go func() {
		var (
			o   []byte
			err error
		)
		if useGoSSH {
			o, err = ssh.RunCommand(j.host, j.command, timeout)
		} else {
			o, err = ssh.RunCommandWithOpenSSH(j.host, j.command)
		}
		done <- &result{
			host:   j.host,
			output: o,
			err:    err,
		}
	}()

	timeoutProxy := make(chan time.Time)
	if timeout != 0 {
		go func() {
			t := <-time.After(timeout)
			timeoutProxy <- t
		}()
	}

	select {
	case r := <-done:
		return r
	case <-timeoutProxy:
		log.Info("Command timed out")

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

func aggregator(results <-chan *result, resultsFinished chan<- struct{}, collapse bool) {
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
