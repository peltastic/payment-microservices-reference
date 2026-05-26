import { Test, TestingModule } from '@nestjs/testing';
import { INestApplication } from '@nestjs/common';
import { HttpService } from '@nestjs/axios';
import { getToken } from '@willsoto/nestjs-prometheus';
import request from 'supertest';
import { of } from 'rxjs';
import { ProxyController } from '../src/proxy/proxy.controller';
import { ProxyService } from '../src/proxy/proxy.service';
import { LoggerService } from '../src/common/filters/logger.service';

describe('gateway proxy integration', () => {
  let app: INestApplication;
  const originalEnv = { ...process.env };
  const httpService = {
    request: jest.fn(() => of({ status: 202, data: { accepted: true } })),
  };

  beforeEach(async () => {
    jest.clearAllMocks();
    process.env = {
      ...originalEnv,
      INTERNAL_AUTH_SECRET: 'integration-secret',
      PAYMENT_SERVICE_URL: 'http://payment.test',
    };
    const moduleFixture: TestingModule = await Test.createTestingModule({
      controllers: [ProxyController],
      providers: [
        ProxyService,
        LoggerService,
        { provide: HttpService, useValue: httpService },
        {
          provide: getToken('payment_reference_proxy_requests_total'),
          useValue: { inc: jest.fn() },
        },
      ],
    }).compile();

    app = moduleFixture.createNestApplication();
    app.use((req: any, _res: any, next: () => void) => {
      req.requestId = 'req_integration';
      req.merchantId = 'mrc_integration';
      req.merchantScope = 'payments:read';
      next();
    });
    await app.init();
  });

  afterEach(async () => {
    process.env = originalEnv;
    await app.close();
  });

  it('given a gateway payments request when routed through Nest then forwards to the payment service contract', async () => {
    // Arrange
    const query = '/v1/payments?status=completed&tag=a&tag=b';

    // Act
    const response = await request(app.getHttpServer()).get(query);

    // Assert
    expect(response.status).toBe(202);
    expect(response.body).toEqual({ accepted: true });
    expect(httpService.request).toHaveBeenCalledWith(
      expect.objectContaining({
        method: 'GET',
        url: 'http://payment.test/payments/?status=completed&tag=a&tag=b',
      }),
    );
  });
});
