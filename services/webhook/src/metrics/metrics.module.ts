import { Global, Module } from '@nestjs/common';
import {
  httpRequestDurationProvider,
  httpRequestsTotalProvider,
  webhookDeadLetterProvider,
  webhookDeliveriesProvider,
  webhookRetryQueueProvider,
} from './metrics';

@Global()
@Module({
  providers: [
    httpRequestsTotalProvider,
    httpRequestDurationProvider,
    webhookDeliveriesProvider,
    webhookDeadLetterProvider,
    webhookRetryQueueProvider,
  ],
  exports: [
    httpRequestsTotalProvider,
    httpRequestDurationProvider,
    webhookDeliveriesProvider,
    webhookDeadLetterProvider,
    webhookRetryQueueProvider,
  ],
})
export class MetricsModule {}
