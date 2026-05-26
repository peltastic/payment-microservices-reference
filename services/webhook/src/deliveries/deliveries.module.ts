import { Module } from '@nestjs/common';
import { BullModule } from '@nestjs/bullmq';
import { PrismaModule } from '../prisma/prisma.module';
import { EndpointsModule } from '../endpoints/endpoints.module';
import { DeliveriesController } from './deliveries.controller';
import { DeliveriesService } from './deliveries.service';
import { DeliveriesRepository } from './deliveries.repository';
import { DeliveryProcessor } from './delivery.processor';

@Module({
  imports: [
    PrismaModule,
    EndpointsModule,
    BullModule.registerQueue({ name: 'webhook-deliveries' }),
  ],
  controllers: [DeliveriesController],
  providers: [DeliveriesService, DeliveriesRepository, DeliveryProcessor],
  exports: [DeliveriesService],
})
export class DeliveriesModule {}
