import { Module } from '@nestjs/common';
import { EventsConsumer } from './events.consumer';
import { EndpointsModule } from '../endpoints/endpoints.module';
import { DeliveriesModule } from '../deliveries/deliveries.module';

@Module({
  imports: [EndpointsModule, DeliveriesModule],
  providers: [EventsConsumer],
})
export class ConsumersModule {}
