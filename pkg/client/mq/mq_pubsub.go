package mq

func NewPubSubPublisher(url string, exchangeName string) (*Publisher, error) {
	exchangeConf := &MQExchangeConf{
		ExchangeName: exchangeName,
		EType:        ExchangeTypeFanout,
		Persistent:   false,
	}

	return NewPublisher(&MQPublisherConf{
		Url:      url,
		Exchange: exchangeConf,
	})
}

func NewPubSubSubscriber(subscriberID string, url string, exchangeName string, handleMsgFun func(string, string, []byte)) (*Consumer, error) {
	conf := &MQConsumerConf{
		Url: url,
		Exchanges: []*MQExchangeConf{
			{
				ExchangeName: exchangeName,
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
						HandleMsgFun:  handleMsgFun,
					},
				},
			},
		},
	}
	return NewConsumer(subscriberID, conf)
}
