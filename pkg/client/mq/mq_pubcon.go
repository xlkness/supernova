package mq

import (
	"context"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func NewPublisher(conf *MQPublisherConf) (*Publisher, error) {
	p := &Publisher{
		exchangeName: conf.Exchange.ExchangeName,
		Persistent:   conf.Exchange.Persistent,
	}

	go func() {
	OUT1:
		for {
			connectionCloseCh := make(chan *amqp.Error, 1)
			connection, err := amqp.Dial(conf.Url)
			if err != nil {
				log.Errorf("[RABBITMQ] dial %v error:%v", conf.Url, err)
				time.Sleep(time.Second * 5)
				continue OUT1
			}

			connection.NotifyClose(connectionCloseCh)

		OUT2:
			for {
				select {
				case <-connectionCloseCh:
					continue OUT1
				default:
				}
				channelCloseCh := make(chan *amqp.Error, 1)
				channel, err := connection.Channel()
				if err != nil {
					log.Errorf("[RABBITMQ] new channel error:%v", err)
					time.Sleep(time.Second * 2)
					continue OUT2
				}

				err = p.init(channel, conf)
				if err != nil {
					connection.Close()
					panic(err)
				}

				channel.NotifyClose(channelCloseCh)

				for {
					select {
					case msg := <-connectionCloseCh:
						log.Errorf("[RABBITMQ] connection close with error:%v", msg.Error())
						continue OUT1
					case msg := <-channelCloseCh:
						log.Errorf("[RABBITMQ] channel close with error:%v", msg.Error())
						continue OUT2
					}
				}
			}
		}
	}()
	time.Sleep(time.Second)
	return p, nil
}

func (p *Publisher) init(ch *amqp.Channel, conf *MQPublisherConf) error {
	var durable, autoDelete bool
	var persistent uint8
	if conf.Exchange.Persistent {
		durable = true
		autoDelete = false
		persistent = amqp.Transient
	} else {
		durable = false
		autoDelete = true
		persistent = amqp.Persistent
	}

	var args amqp.Table
	if conf.Exchange.EArgs != nil {
		args = conf.Exchange.EArgs
	}
	err := ch.ExchangeDeclare(conf.Exchange.ExchangeName, string(conf.Exchange.EType),
		durable, autoDelete, false, false, args)
	if err != nil {
		return err
	}

	p.persistent = persistent
	p.channel = ch
	return nil
}

func NewConsumer(consumerGlobalID string, conf *MQConsumerConf) (*Consumer, error) {
	go func() {
	OUT1:
		for {
			connectionCloseCh := make(chan *amqp.Error, 1)
			connection, err := amqp.Dial(conf.Url)
			if err != nil {
				log.Errorf("[RABBITMQ] dial %v error:%v", conf.Url, err)
				time.Sleep(time.Second * 5)
				continue OUT1
			}

			connection.NotifyClose(connectionCloseCh)

			ctx, cancelFun := context.WithCancel(context.Background())

			for {
				for _, eConf := range conf.Exchanges {
					go func(eConf1 *MQExchangeConf) {
						retryTime := 0
					OUT3:
						for {
							retryTime++
							channelCloseCh := make(chan *amqp.Error, 1)
							channel, err := connection.Channel()
							if err != nil {
								log.Errorf("[RABBITMQ] new channel:%v", err)
								if retryTime > 6 {
									// 主动关闭连接，外层channel监测到connection关闭，调用cancelFun，回收资源
									log.Errorf("[RABBITMQ] new channel reach max times, close connection and reconnect")
									connection.Close()
									return
								}
								time.Sleep(time.Second * 2)
								continue OUT3
							}

							channel.NotifyClose(channelCloseCh)

							newOneChannelConsumer(consumerGlobalID, channel, eConf1, ctx)

							select {
							case msg := <-channelCloseCh:
								log.Errorf("[RABBITMQ] connection close with error:%v", msg.Error())
								continue OUT3
							case <-ctx.Done():
								return
							}
						}
					}(eConf)
				}

				select {
				case msg := <-connectionCloseCh:
					log.Errorf("[RABBITMQ] connection close with error:%v", msg.Error())
					cancelFun()
					continue OUT1
				}
			}
		}
	}()

	return new(Consumer), nil
}

func newOneChannelConsumer(consumerGlobalID string, channel *amqp.Channel, eConf *MQExchangeConf, done context.Context) error {
	// channel.Qos(10,1,false)
	var durable, autoDelete bool
	if eConf.Persistent {
		durable = true
		autoDelete = false
	} else {
		durable = false
		autoDelete = true
	}

	var args amqp.Table
	if eConf.EArgs != nil {
		args = eConf.EArgs
	}
	err := channel.ExchangeDeclare(eConf.ExchangeName, string(eConf.EType), durable, autoDelete,
		false, false, args)
	if err != nil {
		return err
	}

	for i, qConf := range eConf.Queues {
		if qConf.DlxExchange == "" && qConf.HandleMsgFun == nil {
			return fmt.Errorf("queue:%v not found handle message function", qConf.QueueName)
		}

		var arg amqp.Table
		if qConf.DlxExchange != "" {
			arg = amqp.Table{"x-dead-letter-exchange": qConf.DlxExchange}
		}

		queue, err := channel.QueueDeclare(qConf.QueueName, durable, autoDelete,
			qConf.Exclusive, false, arg)
		if err != nil {
			return err
		}

		for _, qBindConf := range qConf.Binds {
			err = channel.QueueBind(queue.Name, qBindConf.BindKey, eConf.ExchangeName, false, nil)
			if err != nil {
				return err
			}
		}

		cn := 1
		if qConf.LBConsumerNum > 0 {
			cn = qConf.LBConsumerNum
		}

		if qConf.DlxExchange == "" {
			for cn > 0 {
				consumerUniqueID := fmt.Sprintf("queue.%v.%v.consumer.%v.%v", qConf.QueueName, i, consumerGlobalID, cn)
				deliveries, err := channel.Consume(queue.Name, consumerUniqueID,
					false, false, false, false, nil)
				if err != nil {
					return err
				}
				go func(handleFun HandleMsgFun) {
					for {
						select {
						case d, ok := <-deliveries:
							if !ok {
								return
							}
							handleFun(consumerUniqueID, d.RoutingKey, d.Body)
							d.Ack(false)
						case <-done.Done():
							return
						}
					}
				}(qConf.HandleMsgFun)
				cn--
			}
		}
	}

	return nil
}
