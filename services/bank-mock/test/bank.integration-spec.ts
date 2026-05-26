import { Test, TestingModule } from '@nestjs/testing';
import { INestApplication } from '@nestjs/common';
import request from 'supertest';
import { AppModule } from '../src/app.module';

describe('bank mock integration', () => {
  let app: INestApplication;

  beforeEach(async () => {
    const moduleFixture: TestingModule = await Test.createTestingModule({
      imports: [AppModule],
    }).compile();

    app = moduleFixture.createNestApplication();
    await app.init();
  });

  afterEach(async () => {
    await app.close();
  });

  it('given a void request when routed through Nest then echoes the voided bank reference', async () => {
    // Arrange
    const reference = 'bank_ref_integration';

    // Act
    const response = await request(app.getHttpServer())
      .post('/bank/void')
      .send({ reference });

    // Assert
    expect(response.status).toBe(201);
    expect(response.body).toEqual({ success: true, voided: reference });
  });
});
