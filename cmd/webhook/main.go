/*
 * Copyright 2019 Open Networking Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
