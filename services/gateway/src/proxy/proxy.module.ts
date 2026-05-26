import { Module } from '@nestjs/common';
import { ProxyController } from './proxy.controller';
import { ProxyService } from './proxy.service';
import { HttpModule } from '@nestjs/axios';

@Module({
  imports: [
    HttpModule.register({
      timeout: Number(process.env.GATEWAY_UPSTREAM_TIMEOUT_MS || 5000),
      maxRedirects: 0,
    }),
  ],
  controllers: [ProxyController],
  providers: [ProxyService],
})
export class ProxyModule {}
