package main

import (
	"flag"
	"github.com/atomix/atomix-sidecar-injector/pkg/webhook"
	"k8s.io/klog/glog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var params webhook.WebhookServerParameters

	flag.IntVar(&params.Port, "port", 443, "Webhook server port.")
	flag.StringVar(&params.CertFile, "tlsCertFile", "/etc/webhook/certs/cert.pem", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&params.KeyFile, "tlsKeyFile", "/etc/webhook/certs/key.pem", "File containing the x509 private key to --tlsCertFile.")
	flag.StringVar(&params.Config, "config", "/etc/webhook/config/atomix.conf", "File containing the Atomix agent configuration to use.")
	flag.Parse()

	wh := webhook.New(params)

	wh.Start()

	glog.Info("Server started")

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	glog.Info("Got OS shutdown signal; shutting down webhook server gracefully...")
	wh.Stop()
}
