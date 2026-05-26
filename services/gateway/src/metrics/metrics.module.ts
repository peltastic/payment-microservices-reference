import { Global, Module } from '@nestjs/common';
import {
  httpRequestsTotalProvider,
  httpRequestDurationProvider,
  authValidationsTotalProvider,
  proxyRequestsTotalProvider,
} from './metrics';

@Global()
@Module({
  providers: [
    httpRequestsTotalProvider,
    httpRequestDurationProvider,
    authValidationsTotalProvider,
    proxyRequestsTotalProvider,
  ],
  exports: [
    httpRequestsTotalProvider,
    httpRequestDurationProvider,
    authValidationsTotalProvider,
    proxyRequestsTotalProvider,
  ],
})
export class MetricsModule {}
