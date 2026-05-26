jest.mock('./deliveries.repository', () => ({
  DeliveriesRepository: class DeliveriesRepository {},
}));

import { DeliveriesService } from './deliveries.service';

describe('DeliveriesService.enqueue', () => {
  const logger = {
    with: jest.fn(() => ({
      info: jest.fn(),
    })),
  };

  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('given a pending delivery when enqueued then creates a BullMQ job with retry backoff', async () => {
    // Arrange
    const delivery = {
      id: 'whd_unit',
      status: 'pending',
      endpointId: 'we_unit',
      merchantId: 'mrc_unit',
      eventId: 'evt_unit',
      eventType: 'payment.succeeded',
    };
    const repo = { create: jest.fn().mockResolvedValue(delivery) };
    const queue = { add: jest.fn().mockResolvedValue({ id: delivery.id }) };
    const service = new DeliveriesService(
      repo as never,
      queue as never,
      logger as never,
    );

    // Act
    const result = await service.enqueue({
      endpointId: 'we_unit',
      merchantId: 'mrc_unit',
      eventId: 'evt_unit',
      eventType: 'payment.succeeded',
      payload: { id: 'evt_unit' },
    });

    // Assert
    expect(result).toBe(delivery);
    expect(queue.add).toHaveBeenCalledWith(
      'deliver',
      { deliveryId: 'whd_unit' },
      expect.objectContaining({
        attempts: 5,
        backoff: { type: 'exponential', delay: 30000 },
        jobId: 'whd_unit',
      }),
    );
  });

  it('given a terminal delivery when enqueued then skips the queue write', async () => {
    // Arrange
    const repo = {
      create: jest
        .fn()
        .mockResolvedValue({ id: 'whd_done', status: 'delivered' }),
    };
    const queue = { add: jest.fn() };
    const service = new DeliveriesService(
      repo as never,
      queue as never,
      logger as never,
    );

    // Act
    await service.enqueue({
      endpointId: 'we_unit',
      merchantId: 'mrc_unit',
      eventId: 'evt_unit',
      eventType: 'payment.succeeded',
      payload: { id: 'evt_unit' },
    });

    // Assert
    expect(queue.add).not.toHaveBeenCalled();
  });
});
