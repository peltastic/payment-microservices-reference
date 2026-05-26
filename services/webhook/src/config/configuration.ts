export default () => ({
  port: parseInt(process.env.PORT || '4004'),
  redis: {
    host: process.env.REDIS_HOST || 'redis',
    port: parseInt(process.env.REDIS_PORT || '6379'),
  },
  kafka: {
    brokers: (process.env.KAFKA_BROKERS || 'kafka:9092').split(','),
    groupId: process.env.KAFKA_GROUP_ID || 'webhook-service',
  },
});
