import { Module } from '@nestjs/common';
import { APP_FILTER, APP_GUARD, APP_INTERCEPTOR } from '@nestjs/core';
import { BullModule } from '@nestjs/bullmq';
import { AppController } from './app.controller';
import { AppService } from './app.service';
import { LoggerModule } from './common/logger.module';
import { HttpExceptionFilter } from './common/filters/http-exception.filter';
import { RequestIdInterceptor } from './common/filters/request-id.interceptor';
import { ConsumersModule } from './consumers/consumers.module';
import { DeliveriesModule } from './deliveries/deliveries.module';
import { EndpointsModule } from './endpoints/endpoints.module';
import { PrismaModule } from './prisma/prisma.module';
import { PrometheusModule } from '@willsoto/nestjs-prometheus';
import { MetricsModule } from './metrics/metrics.module';
import { MetricsInterceptor } from './common/interceptors/metrics.interceptor';
import { InternalAuthGuard } from './security/internal-auth.guard';

@Module({
  imports: [
    LoggerModule,
    PrismaModule,
    PrometheusModule.register({
      path: '/metrics',
      defaultMetrics: {
        enabled: true,
      },
    }),
    MetricsModule,
    BullModule.forRoot({
      connection: {
        host: process.env.REDIS_HOST || 'redis',
        port: Number(process.env.REDIS_PORT || 6379),
        password: process.env.REDIS_PASSWORD || undefined,
      },
    }),
    EndpointsModule,
    DeliveriesModule,
    ConsumersModule,
  ],
  controllers: [AppController],
  providers: [
    AppService,
    {
      provide: APP_FILTER,
      useClass: HttpExceptionFilter,
    },
    {
      provide: APP_GUARD,
      useClass: InternalAuthGuard,
    },
    {
      provide: APP_INTERCEPTOR,
      useClass: RequestIdInterceptor,
    },
    {
      provide: APP_INTERCEPTOR,
      useClass: MetricsInterceptor,
    },
  ],
})
export class AppModule {}
