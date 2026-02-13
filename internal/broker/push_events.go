package broker

const (
	// PushStreamName is a durable stream used for push notification events.
	PushStreamName = "broker:push:events"
	// PushConsumerGroup is a consumer group for push service workers.
	PushConsumerGroup = "push-service"
	// TopicPushNotify sends a web-push notification to one user.
	TopicPushNotify = "push.notify"
	// TopicPushSubscribe stores/updates browser push subscription for user.
	TopicPushSubscribe = "push.subscribe"
	// TopicPushUnsubscribe removes browser push subscription by endpoint.
	TopicPushUnsubscribe = "push.unsubscribe"
)

// PushNotifyPayload is a broker payload for one push notification.
type PushNotifyPayload struct {
	UserID string            `json:"user_id"`
	Title  string            `json:"title"`
	Body   string            `json:"body"`
	Data   map[string]string `json:"data,omitempty"`
}

type PushSubscription struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

type PushSubscribePayload struct {
	UserID       string           `json:"user_id"`
	Subscription PushSubscription `json:"subscription"`
}

type PushUnsubscribePayload struct {
	UserID   string `json:"user_id"`
	Endpoint string `json:"endpoint"`
}
