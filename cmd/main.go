package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/crazy-max/cron"
	"github.com/crazy-max/ddns-route53/internal/app"
	"github.com/crazy-max/ddns-route53/internal/config"
	"github.com/crazy-max/ddns-route53/internal/logging"
	"github.com/rs/zerolog/log"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	ddnsRoute53 *app.Client
	flags       config.Flags
	c           *cron.Cron
	version     = "dev"
)

func main() {
	// Parse command line
	kingpin.Flag("config", "ddns-route53 configuration file.").Envar("CONFIG").Required().StringVar(&flags.Cfgfile)
	kingpin.Flag("schedule", "CRON expression format.").Envar("SCHEDULE").StringVar(&flags.Schedule)
	kingpin.Flag("timezone", "Timezone assigned to ddns-route53.").Envar("TZ").Default("UTC").StringVar(&flags.Timezone)
	kingpin.Flag("log-level", "Set log level.").Envar("LOG_LEVEL").Default("info").StringVar(&flags.LogLevel)
	kingpin.Flag("log-json", "Enable JSON logging output.").Envar("LOG_JSON").Default("false").BoolVar(&flags.LogJson)
	kingpin.UsageTemplate(kingpin.CompactUsageTemplate).Version(version).Author("CrazyMax")
	kingpin.CommandLine.Name = "ddns-route53"
	kingpin.CommandLine.Help = `Dynamic DNS for Amazon Route 53‎ on a time-based schedule. More info on https://github.com/crazy-max/ddns-route53`
	kingpin.Parse()

	// Load timezone location
	location, err := time.LoadLocation(flags.Timezone)
	if err != nil {
		log.Panic().Err(err).Msgf("Cannot load timezone %s", flags.Timezone)
	}

	// Init
	logging.Configure(&flags, location)
	log.Info().Msgf("Starting ddns-route53 %s", version)

	// Handle os signals
	channel := make(chan os.Signal)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-channel
		log.Warn().Msgf("Caught signal %v", sig)
		os.Exit(0)
	}()

	// Load and check configuration
	cfg, err := config.Load(flags, version)
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot load configuration")
	}
	if err := cfg.Check(); err != nil {
		log.Fatal().Err(err).Msg("Improper configuration")
	}

	// Init
	if ddnsRoute53, err = app.New(cfg, location); err != nil {
		log.Fatal().Err(err).Msg("Cannot initialize ddns-route53")
	}

	// Run immediately if schedule is not defined
	if flags.Schedule == "" {
		ddnsRoute53.Run()
		return
	}

	// Start cronjob
	c = cron.NewWithLocation(location)
	log.Info().Msgf("Add cronjob with schedule %s", flags.Schedule)
	if err := c.AddJob(flags.Schedule, ddnsRoute53); err != nil {
		log.Fatal().Err(err).Msg("Cannot create cron task")
	}
	c.Start()

	select {}
}