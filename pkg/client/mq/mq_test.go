package mq

import (
	"fmt"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestTopic(t *testing.T) {
	url := "amqp://likun:123@192.168.1.22:5672/vhost-likun"
	conf1 := &MQConsumerConf{
		Url: url,
		Exchanges: []*MQExchangeConf{
			{
				ExchangeName: "test-persistent-topic-exchange",
				EType:        ExchangeTypeTopic,
				Queues: []*MQQueueConf{
					{
						QueueName: "durable-queue1",
						Binds: []*MQQueueBindConf{
							{"*.create_role_ok"}, {"*.login_role_ok"},
						},
						LBConsumerNum: 2,
						HandleMsgFun: func(consumerUniqueID string, routingKey string, payload []byte) {
							fmt.Printf("[persistent][%v] receive key:%v msg:%v\n",
								consumerUniqueID, routingKey, string(payload))
						},
					},
					{
						QueueName: "durable-queue2",
						Binds: []*MQQueueBindConf{
							{"*.create_role_ok"}, {"*.login_role_ok"},
						},
						LBConsumerNum: 2,
						HandleMsgFun: func(consumerUniqueID string, routingKey string, payload []byte) {
							fmt.Printf("[persistent][%v] receive key:%v msg:%v\n",
								consumerUniqueID, routingKey, string(payload))
						},
					},
				},
			},
			{
				ExchangeName: "test-non-persistent-topic-exchange",
				EType:        ExchangeTypeTopic,
				Persistent:   true,
				Queues: []*MQQueueConf{
					{
						QueueName: "nondurable-queue1",
						Binds: []*MQQueueBindConf{
							{"*.get_legend"}, {"*.win_3_rank"},
						},
						HandleMsgFun: func(consumerUniqueID string, routingKey string, payload []byte) {
							fmt.Printf("[non-persistent][%v] receive key:%v msg:%v\n",
								consumerUniqueID, routingKey, string(payload))
						},
					},
					{
						QueueName: "nondurable-queue2",
						Binds: []*MQQueueBindConf{
							{"*.get_legend"}, {"*.win_3_rank"},
						},
						HandleMsgFun: func(consumerUniqueID string, routingKey string, payload []byte) {
							fmt.Printf("[non-persistent][%v] receive key:%v msg:%v\n",
								consumerUniqueID, routingKey, string(payload))
						},
					},
				},
			},
		},
	}

	_, err := NewConsumer("test-c1", conf1)
	if err != nil {
		panic(err)
	}
	_, err = NewConsumer("test-c2", conf1)
	if err != nil {
		panic(err)
	}

	p1, err := NewPublisher(&MQPublisherConf{
		Url:      url,
		Exchange: conf1.Exchanges[0],
	})
	if err != nil {
		panic(err)
	}

	p2, err := NewPublisher(&MQPublisherConf{
		Url:      url,
		Exchange: conf1.Exchanges[1],
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("init consumers and publishers ok.\n")

	go func() {
		for {
			p1.Publish("game.create_role_ok", []byte("create role ok"))
			// p1.Publish("game.login_role_ok", []byte("login role ok"))
			time.Sleep(time.Second * 3)
			fmt.Printf("\n")
			p2.Publish("game.get_legend", []byte("get legend"))
			// p2.Publish("game.win_3_rank", []byte("win 3 rank"))
			time.Sleep(time.Second * 3)
			fmt.Printf("\n")
		}
	}()

	select {}
}

func TestPubSub(t *testing.T) {
	url := "amqp://likun:123@192.168.1.22:5672/vhost-likun"
	conf1 := &MQConsumerConf{
		Url: url,
		Exchanges: []*MQExchangeConf{
			{
				ExchangeName: "test-persistent-fanout1-exchange",
				EType:        ExchangeTypeFanout,
				Persistent:   false,
				Queues: []*MQQueueConf{
					{
						QueueName: "",
						Binds: []*MQQueueBindConf{
							{""},
						},
						Exclusive:     true,
						LBConsumerNum: 1,
						HandleMsgFun: func(consumerUniqueID string, routingKey string, payload []byte) {
							fmt.Printf("[persistent][%v] receive key:%v msg:%v\n",
								consumerUniqueID, routingKey, string(payload))
						},
					},
				},
			},
		},
	}

	conf1.Exchanges[0].Queues[0].QueueName = "test-c1"
	conf1.Exchanges[0].Queues[0].Binds[0].BindKey = "topic1"
	_, err := NewConsumer("test-c1", conf1)
	if err != nil {
		panic(err)
	}

	conf1.Exchanges[0].Queues[0].QueueName = "test-c2"
	conf1.Exchanges[0].Queues[0].Binds[0].BindKey = "topic2"
	_, err = NewConsumer("test-c2", conf1)
	if err != nil {
		panic(err)
	}

	p1, err := NewPublisher(&MQPublisherConf{
		Url:      url,
		Exchange: conf1.Exchanges[0],
	})
	if err != nil {
		panic(err)
	}

	// p2, err := NewPublisher(&MQPublisherConf{
	// 	Url:      url,
	// 	Exchange: conf1.Exchanges[1],
	// })
	// if err != nil {
	// 	panic(err)
	// }

	fmt.Printf("init consumers and publishers ok.\n")

	go func() {
		for {
			p1.Publish("topic1", []byte("create role ok"))
			// p1.Publish("game.login_role_ok", []byte("login role ok"))
			time.Sleep(time.Second * 3)
			fmt.Printf("\n")
			p1.Publish("topic2", []byte("create role ok"))
			// p1.Publish("game.login_role_ok", []byte("login role ok"))
			time.Sleep(time.Second * 3)
			// p2.Publish("", []byte("get legend"))
			// // p2.Publish("game.win_3_rank", []byte("win 3 rank"))
			// time.Sleep(time.Second * 3)
			// fmt.Printf("\n")
		}
	}()

	select {}
}

func TestDlxCron(t *testing.T) {
	url := "amqp://likun:123@192.168.1.22:5672/vhost-likun"
	receiverExConf := &MQExchangeConf{
		ExchangeName: "test.dlx.cron.ex1",
		EType:        ExchangeTypeDirect,
		Persistent:   true,
		Queues:       []*MQQueueConf{},
	}

	receiverExConf1 := &MQExchangeConf{
		ExchangeName: "test.dlx.cron.ex1",
		EType:        ExchangeTypeDirect,
		Persistent:   true,
		Queues: []*MQQueueConf{
			{
				QueueName: "test.dlx.cron.queue1",
				Binds: []*MQQueueBindConf{
					{"weekly"},
				},
				DlxExchange: "test.dlx.cron.ex2",
			},
		},
	}

	publisherExConf := &MQExchangeConf{
		ExchangeName: "test.dlx.cron.ex2",
		EType:        ExchangeTypeDirect,
		Persistent:   true,
		Queues: []*MQQueueConf{
			{
				QueueName: "test.dlx.cron.queue1",
				Binds: []*MQQueueBindConf{
					{"weekly"},
				},
				DlxExchange: "test.dlx.cron.ex2",
			},
			{
				QueueName: "test.dlx.cron.queue2",
				Binds: []*MQQueueBindConf{
					{"weekly"},
				},
				HandleMsgFun: func(consumerUniqueID string, routingKey string, payload []byte) {
					fmt.Printf("[%v][persistent][%v] receive key:%v msg:%v\n",
						time.Now().String(), consumerUniqueID, routingKey, string(payload))
				},
			},
		},
	}

	conf := &MQPublisherConf{
		Url:      url,
		Exchange: receiverExConf,
	}
	conf1 := &MQConsumerConf{
		Url:       url,
		Exchanges: []*MQExchangeConf{receiverExConf1, publisherExConf},
	}

	pub, err := NewPublisher(conf)
	if err != nil {
		panic(err)
	}

	_, err = NewConsumer("expire.consumer.1", conf1)
	if err != nil {
		panic(err)
	}
	_, err = NewConsumer("expire.consumer.2", conf1)
	if err != nil {
		panic(err)
	}

	pub.PublishEx("weekly", []byte("test weekly payload"), time.Second*10)
	fmt.Printf("[%v] publish expire msg ok.\n", time.Now().String())
	select {}
}

type A struct {
	Field1 string
	Field2 int
}

type B struct {
	Field1 string
	Field2 interface{}
}

func TestXConsistentExchange(t *testing.T) {
	url := "amqp://likun:123@192.168.1.22:5672/vhost-likun"
	exchangeName := "test.xconsistent.ex"
	count1 := int32(0)
	count2 := int32(0)
	conf1 := &MQConsumerConf{
		Url: url,
		Exchanges: []*MQExchangeConf{
			{
				ExchangeName: exchangeName,
				EType:        ExchangeTypeXConsistent,
				EArgs:        amqp.Table{"hash-header": "hash-on"},
				Persistent:   false,
				Queues: []*MQQueueConf{
					{
						QueueName: "",
						Binds: []*MQQueueBindConf{
							{"1"},
						},
						Exclusive:     false,
						LBConsumerNum: 1,
						HandleMsgFun: func(consumerUniqueID string, routingKey string, payload []byte) {
							// fmt.Printf("[persistent][%v] receive key:%v msg:%v\n",
							// 	consumerUniqueID, routingKey, string(payload))
							fmt.Printf("1:%v\n", atomic.AddInt32(&count1, 1))
						},
					},
				},
			},
		},
	}

	conf2 := &MQConsumerConf{
		Url: url,
		Exchanges: []*MQExchangeConf{
			{
				ExchangeName: exchangeName,
				EType:        ExchangeTypeXConsistent,
				EArgs:        amqp.Table{"hash-header": "hash-on"},
				Persistent:   false,
				Queues: []*MQQueueConf{
					{
						QueueName: "",
						Binds: []*MQQueueBindConf{
							{"1"},
						},
						Exclusive:     false,
						LBConsumerNum: 1,
						HandleMsgFun: func(consumerUniqueID string, routingKey string, payload []byte) {
							// fmt.Printf("[persistent][%v] receive key:%v msg:%v\n",
							// 	consumerUniqueID, routingKey, string(payload))
							fmt.Printf("2:%v\n", atomic.AddInt32(&count2, 1))
						},
					},
				},
			},
		},
	}

	conf1.Exchanges[0].Queues[0].QueueName = "test-c1"
	conf1.Exchanges[0].Queues[0].Binds[0].BindKey = "1"
	_, err := NewConsumer("test-c1", conf1)
	if err != nil {
		panic(err)
	}

	conf2.Exchanges[0].Queues[0].QueueName = "test-c2"
	conf2.Exchanges[0].Queues[0].Binds[0].BindKey = "1"
	_, err = NewConsumer("test-c2", conf2)
	if err != nil {
		panic(err)
	}

	p1, err := NewPublisher(&MQPublisherConf{
		Url:      url,
		Exchange: conf1.Exchanges[0],
	})
	if err != nil {
		panic(err)
	}

	time.Sleep(time.Second * 2)
	// p2, err := NewPublisher(&MQPublisherConf{
	// 	Url:      url,
	// 	Exchange: conf1.Exchanges[1],
	// })
	// if err != nil {
	// 	panic(err)
	// }

	fmt.Printf("init consumers and publishers ok.\n")

	for i := 0; i < 100000; i++ {
		p1.PublishXConsistent("topic"+strconv.Itoa(i), "topic", []byte("create role ok"))
	}

	time.Sleep(time.Second * 10)
	fmt.Printf("1:%v,2:%v\n", count1, count2)

	// go func() {
	// 	for {
	// 		for i := 0; i < 100000; i++ {
	// 			p1.PublishXConsistent("topic"+strconv.Itoa(i), []byte("create role ok"))
	// 		}
	// p1.PublishXConsistent("topic2", []byte("create role ok"))
	// p1.PublishXConsistent("topic3", []byte("create role ok"))
	// p1.PublishXConsistent("topic4", []byte("create role ok"))
	// p1.PublishXConsistent("topic5", []byte("create role ok"))
	// p1.PublishXConsistent("topic6", []byte("create role ok"))
	// p1.PublishXConsistent("topic7", []byte("create role ok"))
	// p1.PublishXConsistent("topic8", []byte("create role ok"))
	// p1.Publish("game.login_role_ok", []byte("login role ok"))
	// time.Sleep(time.Second * 3)
	// fmt.Printf("\n")
	// p2.Publish("", []byte("get legend"))
	// // p2.Publish("game.win_3_rank", []byte("win 3 rank"))
	// time.Sleep(time.Second * 3)
	// fmt.Printf("\n")
	// }
	// }()

	select {}
}
