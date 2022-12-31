package main

import (
	"encoding/json"
	"flag"
	"net/http"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	_ "github.com/influxdata/influxdb1-client" // this is important because of the bug in go mod
	influx "github.com/influxdata/influxdb1-client/v2"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

var (
	heatingEventsReceived = promauto.NewCounter(prometheus.CounterOpts{
		Name: "tasmota.sensor.heating.values.received",
		Help: "The total number of received events",
	})
	powerEventsReceived = promauto.NewCounter(prometheus.CounterOpts{
		Name: "tasmota.sensor.power.values.received",
		Help: "The total number of received events",
	})
)

func main() {
	mqttAddress := flag.String("mqtt-address", "192.168.2.3:1883", "mqtt broker")
	influxAddress := flag.String("influx-address", "http://192.168.2.3:8086", "influx broker")
	logrus.Infof("using mqtt %s", *mqttAddress)
	logrus.Infof("using influx %s", *influxAddress)

	influxClient, _ := influx.NewHTTPClient(influx.HTTPConfig{
		Addr:     *influxAddress,
		Username: "admin",
		Password: "admin",
	})

	clientOptions := mqtt.NewClientOptions().AddBroker(*mqttAddress).SetClientID("tasmota")

	mqttClient := mqtt.NewClient(clientOptions)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		logrus.Fatal(token.Error())
	}

	if token := mqttClient.Subscribe("tele/#", 2, onMessage(influxClient)); token.Wait() && token.Error() != nil {
		logrus.Fatal(token.Error())
	}

	defer mqttClient.Disconnect(1000)
	http.Handle("/metrics", promhttp.Handler())
	_ = http.ListenAndServe(":2112", nil)
}

func onMessage(influxClient influx.Client) func(client mqtt.Client, message mqtt.Message) {
	return func(client mqtt.Client, message mqtt.Message) {
		switch message.Topic() {
		case "tele/tasmota_FA2642/SENSOR":
			logrus.Infof("Topic: %s Payload: %s", message.Topic(), message.Payload())
			err := sendSensorPayload("Power", message, influxClient)
			switch err {
			case nil:
				powerEventsReceived.Inc()
			default:
				logrus.Errorf("error writing point: %s", err)
			}

		case "tele/tasmota_5BEF46/SENSOR":
			logrus.Infof("Topic: %s Payload: %s", message.Topic(), message.Payload())
			err := sendSensorPayload("Heating", message, influxClient)
			switch err {
			case nil:
				heatingEventsReceived.Inc()
			default:
				logrus.Errorf("error writing point: %s", err)
			}
		}
	}
}

func sendSensorPayload(counter string, message mqtt.Message, writeApi influx.Client) error {
	var sensor SensorPayload
	err := json.Unmarshal(message.Payload(), &sensor)
	if err != nil {
		return errors.WithMessagef(err, "parse sensor payload: %s", err)
	}
	fields := map[string]any{
		"counter": counter,
		"totalIn": sensor.MT681.TotalIn,
		"current": sensor.MT681.PowerCur,
		"meterId": sensor.MT681.MeterId,
	}
	p, _ := influx.NewPoint("MT681", map[string]string{"counter": counter}, fields, time.Now())
	batch, _ := influx.NewBatchPoints(influx.BatchPointsConfig{Database: "db0"})
	batch.AddPoint(p)
	return writeApi.Write(batch)
}

type SensorPayload struct {
	Time  string `json:"Time"`
	MT681 struct {
		TotalIn  float64 `json:"Total_in"`
		PowerCur int     `json:"Power_cur"`
		PowerP1  int     `json:"Power_p1"`
		PowerP2  int     `json:"Power_p2"`
		PowerP3  int     `json:"Power_p3"`
		TotalOut float64 `json:"Total_out"`
		MeterId  string  `json:"Meter_id"`
	} `json:"MT681"`
}
