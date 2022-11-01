package mq

import (
	"context"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type ConsumerV1 struct {
	consumerGlobalID string
	conn             *amqp.Connection
	channel          *amqp.Channel
	dsn              string
	tag              string
	done             chan error
	handlers         []*MQExchangeConf
}

func NewConsumerV1(consumerGlobalID string, dsn string) (*ConsumerV1, error) {
	c := new(ConsumerV1)
	c.consumerGlobalID = consumerGlobalID
	c.dsn = dsn
	return c, nil
}

func (c *ConsumerV1) Subscribe(eConf *MQExchangeConf) {
	c.handlers = append(c.handlers, eConf)
}

func (c *ConsumerV1) Run() {
	go func() {
	OUT1:
		for {
			connectionCloseCh := make(chan *amqp.Error, 1)
			connection, err := amqp.Dial(c.dsn)
			if err != nil {
				log.Errorf("[RABBITMQ] dial %v error:%v", c.dsn, err)
				time.Sleep(time.Second * 5)
				continue OUT1
			}

			connection.NotifyClose(connectionCloseCh)

			ctx, cancelFun := context.WithCancel(context.Background())

			for {
				for _, eConf := range c.handlers {
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

							c.newOneChannelConsumer(c.consumerGlobalID, channel, eConf1, ctx)

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
}

func (c *ConsumerV1) newOneChannelConsumer(consumerGlobalID string, channel *amqp.Channel, eConf *MQExchangeConf, done context.Context) error {
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
