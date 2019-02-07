package cmd

import (
	"fmt"
	"os"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	command string
	hostsArg      string
	file          string
	maxFlight     int
	timeout       int
	globalTimeout int
	collapse      bool
	verbose bool
	debug bool
)

type job struct {
	host *host
	command string
}

type result struct {
	output []byte
	err error
}

func init() {
	rootCmd.PersistentFlags().StringVar(&hostsArg, "hosts", "", "List of hostnames to execute on")
	rootCmd.PersistentFlags().StringVarP(&file, "file", "f", "", "List of hostnames in a file (/dev/stdin for reading from stdin)")
	rootCmd.PersistentFlags().IntVarP(&maxFlight, "maxflight", "m", 50, "Maximum number of concurrent connections")
	rootCmd.PersistentFlags().IntVarP(&timeout, "timeout", "t", 60, "How many seconds may each individual call take? 0 for no timeout")
	rootCmd.PersistentFlags().IntVarP(&globalTimeout, "timeout_global", "g", 600, "How many seconds for all calls to take? 0 for no timeout")
	rootCmd.PersistentFlags().BoolVarP(&collapse, "collapse", "c", false, "Collapse similar output")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output (INFO level)")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Debug output (DEBUG level)")
}

var rootCmd = &cobra.Command{
	Use:   "mssh [command]",
	Short: "A tool for running multiple commands and ssh jobs in parallel, and easily collecting the results",
	Args: cobra.ExactArgs(1),
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
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		jobs := make(chan *job, maxFlight)
		shutdown := make(chan struct{})
		wg := &sync.WaitGroup{}

		hosts, err := parseHostsArg(hostsArg)
		if err != nil {
			panic(err)
		}

		// No point in extra goroutines
		if len(hosts) < maxFlight {
			maxFlight = len(hosts)
		}

		wg.Add(maxFlight)
		for i := 0; i < maxFlight; i++ {
			go executor(jobs, shutdown, wg)
		}

		// TODO: implement timeouts
		for _, h := range hosts {
			log.WithField("host", h.hostName).Debug("Creating job for host")
			jobs <- &job{
				host: h,
				command: command,
			}
			close(jobs)
		}
		wg.Wait()
	},
}

func executor(queue <-chan *job, shutdown <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case j, ok := <- queue:
			if !ok {
				return
			}
			log.WithField("host", j.host.hostName).Debug("Received job from queue")
			if r := handleJob(j); r != nil && r.err != nil {
				// TODO: send it to an aggregator
				panic(r.err)
			}
		case <- shutdown:
			// TODO: handle this gracefully so you know what commands have finished
			return
		}
	}
}

func handleJob(j *job) *result {
	logger := log.WithField("host", j.host.hostName)
	logger.Info("Connecting to host")
	h, err := connectToHost(j.host)
	if err != nil {
		return &result{err: err}
	}
	defer h.Close()

	logger.Info("Establishing new session")
	s, err := h.NewSession()
	if err != nil {
		return &result{err: err}
	}
	defer s.Close()

	logger.WithField("command", j.command).Info("Running command")
	o, err := s.CombinedOutput(j.command)
	return &result{
		output: o,
		err: err,
	}
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
