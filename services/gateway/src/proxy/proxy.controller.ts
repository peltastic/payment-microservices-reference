import { All, Controller, Req, Res } from '@nestjs/common';
import type { Request, Response } from 'express';
import { ProxyService } from './proxy.service';

type GatewayRequest = Request & {
  requestId?: string;
  merchantId?: string;
  merchantScope?: string;
};

@Controller('v1')
export class ProxyController {
  constructor(private readonly proxyService: ProxyService) {}

  @All('auth/*path')
  async auth(@Req() req: GatewayRequest, @Res() res: Response) {
    const path = this.downstreamPath(req, '/v1/auth');
    const { status, data } = await this.proxyService.forward(
      req,
      'auth',
      `${process.env.AUTH_SERVICE_URL}${path}`,
    );
    res.status(status).json(data);
  }

  @All(['payments', 'payments/*path'])
  async payments(@Req() req: GatewayRequest, @Res() res: Response) {
    const path = this.withTrailingSlash(
      this.downstreamPath(req, '/v1'),
      '/payments',
    );
    const { status, data } = await this.proxyService.forward(
      req,
      'payment',
      `${process.env.PAYMENT_SERVICE_URL}${path}`,
    );
    res.status(status).json(data);
  }

  @All(['balance', 'balance/*path', 'transactions/*path'])
  async ledger(@Req() req: GatewayRequest, @Res() res: Response) {
    const path = this.withTrailingSlash(
      this.downstreamPath(req, '/v1'),
      '/balance',
    );
    const { status, data } = await this.proxyService.forward(
      req,
      'ledger',
      `${process.env.LEDGER_SERVICE_URL}${path}`,
    );
    res.status(status).json(data);
  }

  @All(['webhooks', 'webhooks/*path'])
  async webhooks(@Req() req: GatewayRequest, @Res() res: Response) {
    const path = this.downstreamPath(req, '/v1/webhooks');
    const { status, data } = await this.proxyService.forward(
      req,
      'webhook',
      `${process.env.WEBHOOK_SERVICE_URL}${path}`,
    );
    res.status(status).json(data);
  }

  private downstreamPath(req: Request, prefix: string): string {
    const pathname = (req.originalUrl || req.url).split('?')[0];
    return pathname.replace(prefix, '') || '/';
  }

  private withTrailingSlash(path: string, rootPath: string): string {
    return path === rootPath ? `${rootPath}/` : path;
  }
}
