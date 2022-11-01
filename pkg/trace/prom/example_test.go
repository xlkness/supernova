package prom

import (
	"math/rand"
	"testing"
	"time"
)

func TestProm(t *testing.T) {

	counter := NewCounter("MysqlWriteErr")

	counter.InitDefaultLabels(map[string]string{"default": "label"}, []string{"op", "service"})

	gauge := NewGauge("CCU").InitLabels([]string{"service"})

	hist := NewHistogram("HandleRequest", []float64{100, 200, 300, 400, 1000, 2000, 3000, 4000}).InitLabels([]string{"request"})

	ops := []string{"update", "insert"}
	services := []string{"game", "mail"}
	services1 := []string{"gateway-1", "gateway-2"}

	requests := []string{"Login", "ReadMail", "ShopBuy"}

	go func() {
		for {
			time.Sleep(time.Second * 2)
			counter.LabelValues(ops[rand.Intn(len(ops))], services[rand.Intn(len(services))]).Inc()
			gauge.LabelValues(services1[rand.Intn(len(services1))]).Set(float64(rand.Intn(100) + 20))
			hist.LabelValues(requests[rand.Intn(len(requests))]).Observe(float64(rand.Intn(6000)))
		}
	}()

	go func() {
		time.Sleep(time.Second * 20)
		counter := NewCounter("RedisWriteErr").InitLabels([]string{
			"op", // update/insert
			"service",
		})
		services := []string{"game", "mail"}
		for {
			time.Sleep(time.Second * 2)
			counter.LabelValues(ops[rand.Intn(len(ops))], services[rand.Intn(len(services))]).Inc()
		}
	}()

	RunServer(":9008", true)
	select {}
}
