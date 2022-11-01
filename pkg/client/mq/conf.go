package mq

import (
	"fmt"
	"os"
	"strconv"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type ExchangeType string

const (
	ExchangeTypeDirect      ExchangeType = "direct"
	ExchangeTypeFanout      ExchangeType = "fanout"
	ExchangeTypeTopic       ExchangeType = "topic"
	ExchangeTypeXConsistent ExchangeType = "x-consistent-hash"
)

/*
	MQExchangeConf:交换机配置

	durable&non-auto-deleted:将在mq服务重启后保留交换机声明，就算没有队列绑定。
	对于像稳定路由和默认交换这样的长期交换配置，这是最好的生命周期。

	non-durable&auto-deleted:如果没有队列绑定或者mq服务重启就会自动删除。
	这个生命周期对于在故障时或使用者不用了后不应该污染虚拟主机的临时拓扑是有用的。

	non-durable&non-auto-deleted:在服务器运行期间(包括没有绑定队列时)，交换机将保持不变（mq服务重启就删除了）。
	这对于绑定之间可能有很长时间延迟的临时拓扑非常有用。

	durable&auto-deleted:将在服务器重新启动后继续存在，并且在服务器重新启动之前和之后没有队列绑定时将被删除。
	这些交换对于健壮的临时拓扑或需要将持久队列绑定到自动删除的交换机非常有用。
*/
type MQExchangeConf struct {
	ExchangeName string
	EType        ExchangeType
	EArgs        map[string]interface{}
	Persistent   bool // 是否持久化
	// Durable          bool           // true持久化交换机，用于mq服务重启后客户端依然能继续发送消息而不用重新声明交换机
	// DeleteWhenUnused bool           // true表示没有队列或者交换机与这个交换机绑定，就自动删除交换机。
	Queues []*MQQueueConf // 绑定的队列
	// Internal         bool // true表示内置交换机，客户端无法直接发送消息到这个交换机，而只能通过交换机发给交换机
	// NoWait           bool // true表示不等待服务器返回消息，函数将返回nil，提高速度
}

/*
	MQQueueConf:队列配置

	durable&non-auto-deleted:将在mq服务器重启后继续存在，并在没有剩余消费者或绑定时继续存在。
	设置了Persistent的消息将在服务器重启时在该队列中恢复。这些队列只能绑定到持久交换器。

	non-durable&auto-deleted:不会在mq服务器重启时重新声明，
	并且会在最后一个使用者被取消或最后一个使用者的通道被关闭后的短时间内被服务器删除。
	具有此生存期的队列也可以使用queueddelete正常删除。这些持久性队列只能绑定到非持久性交换。

	non-durable&non-auto-deleted:在服务器运行期间，不管有多少消费者，队列都将保持声明状态。
	这个生存期对于在使用者活动之间可能有长时间延迟的临时拓扑很有用。这些队列只能绑定到非持久交换器。

	durable&auto-deleted:持久和自动删除的队列将在服务器重启时恢复，但没有活动消费者的队列将无法存活并被删除。这一生不太可能有用。
*/
type MQQueueConf struct {
	QueueName string
	// Durable          bool               // true表示持久化，会将queue落盘，mq服务重启
	// DeleteWhenUnused bool               // true表示没有跟这个队列绑定的连接，就自动删除队列，当消费者宕机重启后，由于队列删除，宕机期间的消息丢失
	Binds         []*MQQueueBindConf // 队列绑定了哪些key到交换机
	LBConsumerNum int                // 负载均衡的消费者数量，一般默认1个就行
	HandleMsgFun  HandleMsgFun
	Exclusive     bool // true表示排他队列，该队列只对首次声明他的连接可见，并在连接断开时自动删除，同一连接不同channel是可以访问队列的，一般用于pubsub
	// NoWait           bool // true表示不等待服务器返回消息，函数将返回nil，提高速度
	// Args amqp.Table // 参数，目前只用来声明死信队列时候用到x-dead-letter-exchange
	DlxExchange string // 需要绑定死信交换机，目前只有定时任务需要绑定
}

type MQQueueBindConf struct {
	BindKey string // 队列绑定的key，交换机会根据队列绑定的key来决定投递消息
}

type MQPublisherConf struct {
	Url      string // amqp://likun:123@192.168.1.22:5672/vhost-likun格式
	Exchange *MQExchangeConf
}

type MQConsumerConf struct {
	Url       string // amqp://likun:123@192.168.1.22:5672/vhost-likun格式
	Exchanges []*MQExchangeConf
}

type Logger interface {
	Infof(v ...interface{})
	Errorf(v ...interface{})
}

var log Logger = &defaultLogger{}

func SetLogger(l Logger) {
	log = l
}

type defaultLogger struct {
}

func (*defaultLogger) Infof(v ...interface{}) {

}
func (*defaultLogger) Errorf(v ...interface{}) {
	fmt.Fprint(os.Stderr, v...)
}

type Publisher struct {
	Persistent   bool   // 标识是否持久化交换机和队列，调用Publish接口也会根据标识选择消息持久化方案
	persistent   uint8  // 根据Persistent的值，调用Publish接口时填入deliver_mode
	exchangeName string // 交换机名字
	channel      *amqp.Channel
}

func (p *Publisher) PublishNonPersistent(topic string, data []byte) error {
	return p.publish(topic, data, amqp.Transient, 0, nil)
}

func (p *Publisher) Publish(topic string, data []byte) error {
	return p.publish(topic, data, p.persistent, 0, nil)
}

// PublishEx 推送过期消息，过期时间毫秒
func (p *Publisher) PublishEx(topic string, data []byte, expire time.Duration) error {
	return p.publish(topic, data, p.persistent, expire, nil)
}

// PublishXConsistent 推送一致性hash事件，hashKey作为hash的键，topic是事件名，不参与hash
// 例如：role.login.ok事件中，role_id作为hashKey，role.login.ok作为事件
func (p *Publisher) PublishXConsistent(hashKey string, topic string, data []byte) error {
	return p.publish(topic, data, p.persistent, 0, amqp.Table{"hash-on": hashKey})
}

func (p *Publisher) publish(topic string, data []byte, persistent uint8, expire time.Duration, args amqp.Table) error {
	mills := expire.Milliseconds()

	exp := ""
	if mills > 0 {
		exp = strconv.Itoa(int(mills))
	}
	err := p.channel.Publish(p.exchangeName, topic, false, false, amqp.Publishing{
		Headers:         args,
		ContentType:     "text/plain",
		ContentEncoding: "",
		Body:            data,
		DeliveryMode:    persistent,
		Priority:        0,
		Expiration:      exp,
	})
	return err
}

type Consumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	tag     string
	done    chan error
}

type HandleMsgFun func(consumerID string, routingKey string, payload []byte)
