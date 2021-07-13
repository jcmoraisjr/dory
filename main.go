/*
Copyright 2021 Joao Morais

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	paramBuckets = flag.String("buckets", "0.8,1,1.2", "response time counter buckets")
	paramListen  = flag.String("listen", ":8000", "listening ip:port")
)

var (
	responseTime *prometheus.HistogramVec
	sessionsCnt  *int32
)

func createMetrics() {
	var buckets []float64
	for _, bucket := range strings.Split(*paramBuckets, ",") {
		value, err := strconv.ParseFloat(bucket, 64)
		if err != nil {
			log.Fatal(err)
		}
		buckets = append(buckets, value)
	}

	namespace := "dory"
	responseTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "response_time_seconds",
			Help:      "Response time of the functionality",
			Buckets:   buckets,
		},
		[]string{"waitref"},
	)
	prometheus.MustRegister(*responseTime)

	sessions := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "sessions",
		Help:      "Concurrent sessions",
	}, func() float64 {
		return float64(*sessionsCnt)
	})
	prometheus.MustRegister(sessions)
}

func main() {
	flag.Parse()
	createMetrics()
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}
	sessionsCnt = new(int32)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		atomic.AddInt32(sessionsCnt, 1)
		pct, _ := strconv.Atoi(r.Header.Get("x-pct"))
		wait, _ := strconv.Atoi(r.Header.Get("x-wait"))
		realPct := rand.Float32()*(2*float32(pct)/100) + (1 - (float32(pct) / 100))
		realWait := time.Duration(realPct * float32(wait) * float32(time.Millisecond))
		time.Sleep(realWait)
		fmt.Fprintf(w, hostname+"\n")
		responseTime.WithLabelValues(strconv.Itoa(wait)).Observe(time.Now().Sub(start).Seconds())
		atomic.AddInt32(sessionsCnt, -1)
	})
	http.Handle("/metrics", promhttp.Handler())
	fmt.Println(*paramListen)
	log.Fatal(http.ListenAndServe(*paramListen, nil))
}
