import { Module } from '@nestjs/common';
import { PrismaModule } from '../prisma/prisma.module';
import { EndpointsController } from './endpoints.controller';
import { EndpointsService } from './endpoints.service';
import { EndpointsRepository } from './endpoints.repository';

@Module({
  imports: [PrismaModule],
  controllers: [EndpointsController],
  providers: [EndpointsService, EndpointsRepository],
  exports: [EndpointsService, EndpointsRepository],
})
export class EndpointsModule {}
