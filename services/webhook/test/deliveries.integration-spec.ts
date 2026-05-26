jest.mock('../src/deliveries/deliveries.repository', () => ({
  DeliveriesRepository: class DeliveriesRepository {},
}));

import { Test, TestingModule } from '@nestjs/testing';
import { INestApplication } from '@nestjs/common';
import request from 'supertest';
import { DeliveriesController } from '../src/deliveries/deliveries.controller';
import { DeliveriesService } from '../src/deliveries/deliveries.service';
import { LoggerService } from '../src/common/filters/logger.service';
import { DeliveriesRepository } from '../src/deliveries/deliveries.repository';
import { getQueueToken } from '@nestjs/bullmq';

describe('webhook deliveries integration', () => {
  let app: INestApplication;
  const repo = {
    findAllByMerchant: jest.fn(),
    findDeadLetters: jest.fn(),
    findByIdForMerchant: jest.fn(),
  };
  const queue = { add: jest.fn() };

  beforeEach(async () => {
    jest.clearAllMocks();
    repo.findAllByMerchant.mockResolvedValue({
      data: [{ id: 'whd_integration', status: 'pending' }],
      total: 1,
      page: 2,
      limit: 1,
    });
    repo.findDeadLetters.mockResolvedValue([]);
    repo.findByIdForMerchant.mockResolvedValue({
      id: 'whd_integration',
      merchantId: 'mrc_integration',
    });

    const moduleFixture: TestingModule = await Test.createTestingModule({
      controllers: [DeliveriesController],
      providers: [
        DeliveriesService,
        LoggerService,
        { provide: DeliveriesRepository, useValue: repo },
        { provide: getQueueToken('webhook-deliveries'), useValue: queue },
      ],
    }).compile();

    app = moduleFixture.createNestApplication();
    await app.init();
  });

  afterEach(async () => {
    await app.close();
  });

  it('given a merchant deliveries request when handled through HTTP then returns repository pagination', async () => {
    // Arrange
    const merchantId = 'mrc_integration';

    // Act
    const response = await request(app.getHttpServer())
      .get('/deliveries?page=2&limit=1')
      .set('x-merchant-id', merchantId);

    // Assert
    expect(response.status).toBe(200);
    expect(response.body).toMatchObject({
      total: 1,
      page: 2,
      limit: 1,
    });
    expect(repo.findAllByMerchant).toHaveBeenCalledWith(merchantId, 2, 1);
  });

  it('given a delivery retry request when handled through HTTP then enqueues a retry job', async () => {
    // Arrange
    const merchantId = 'mrc_integration';

    // Act
    const response = await request(app.getHttpServer())
      .post('/deliveries/whd_integration/retry')
      .set('x-merchant-id', merchantId);

    // Assert
    expect(response.status).toBe(201);
    expect(queue.add).toHaveBeenCalledWith(
      'deliver',
      { deliveryId: 'whd_integration' },
      expect.objectContaining({
        attempts: 5,
        jobId: expect.stringContaining('manual-retry:whd_integration:'),
      }),
    );
  });
});
