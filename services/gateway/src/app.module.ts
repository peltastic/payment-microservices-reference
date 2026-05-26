import { Module } from '@nestjs/common';
import { HttpModule } from '@nestjs/axios';
import { AppController } from './app.controller';
import { AppService } from './app.service';
import { APP_FILTER, APP_GUARD, APP_INTERCEPTOR } from '@nestjs/core';
import { AuthGuard } from './guards/auth.guard';
import { ProxyModule } from './proxy/proxy.module';
import { ConfigModule } from '@nestjs/config';
import { LoggerModule } from './common/logger.module';
import { HttpExceptionFilter } from './common/filters/http-exception.filter';
import { RequestIdInterceptor } from './interceptors/request-id.interceptor';
import { PrometheusModule } from '@willsoto/nestjs-prometheus';
import { MetricsModule } from './metrics/metrics.module';
import { MetricsInterceptor } from './interceptors/metrics.interceptor';
import { RateLimitGuard } from './guards/rate-limit.guard';

@Module({
  imports: [
    LoggerModule,
    PrometheusModule.register({
      path: '/metrics',
      defaultMetrics: {
        enabled: true,
      },
    }),
    MetricsModule,
    ConfigModule.forRoot({
      envFilePath: `.env.${process.env.NODE_ENV || 'development'}`,
    }),
    HttpModule.register({
      timeout: Number(process.env.GATEWAY_UPSTREAM_TIMEOUT_MS || 5000),
      maxRedirects: 0,
    }),
    ProxyModule,
  ],
  controllers: [AppController],
  providers: [
    AppService,
    MetricsInterceptor,
    { provide: APP_GUARD, useClass: RateLimitGuard },
    { provide: APP_GUARD, useClass: AuthGuard },
    { provide: APP_FILTER, useClass: HttpExceptionFilter },
    { provide: APP_INTERCEPTOR, useClass: RequestIdInterceptor },
  ],
})
export class AppModule {}
