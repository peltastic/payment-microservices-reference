import { Test, TestingModule } from '@nestjs/testing';
import { INestApplication } from '@nestjs/common';
import request from 'supertest';
import { App } from 'supertest/types';
import { AppModule } from './../src/app.module';

describe('AppController (e2e)', () => {
  let app: INestApplication<App>;
  const originalEnv = { ...process.env };

  beforeEach(async () => {
    process.env = {
      ...originalEnv,
      AUTH_SERVICE_URL: 'http://auth:4001/api/v1/auth',
      PAYMENT_SERVICE_URL: 'http://payment:4002',
      LEDGER_SERVICE_URL: 'http://ledger:4003',
      WEBHOOK_SERVICE_URL: 'http://webhook:4004',
      INTERNAL_AUTH_SECRET: 'e2e-secret',
    };
    const moduleFixture: TestingModule = await Test.createTestingModule({
      imports: [AppModule],
    }).compile();

    app = moduleFixture.createNestApplication();
    await app.init();
  });

  it('given a health request when the gateway app is running then returns healthy status', () => {
    return request(app.getHttpServer())
      .get('/health')
      .expect(200)
      .expect('Hello World!');
  });

  afterEach(async () => {
    process.env = originalEnv;
    await app.close();
  });
});
