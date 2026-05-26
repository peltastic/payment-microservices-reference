import { BadRequestException } from '@nestjs/common';
import { assertSafeWebhookUrl } from './url-safety';

describe('assertSafeWebhookUrl', () => {
  it('given an http webhook url when validated then rejects it before any delivery is stored', async () => {
    // Arrange
    const url = 'http://merchant.example/webhook';

    // Act
    const assertion = assertSafeWebhookUrl(url);

    // Assert
    await expect(assertion).rejects.toBeInstanceOf(BadRequestException);
  });

  it('given a private ipv4 webhook url when validated then rejects it as unsafe', async () => {
    // Arrange
    const url = 'https://192.168.1.10/webhook';

    // Act
    const assertion = assertSafeWebhookUrl(url);

    // Assert
    await expect(assertion).rejects.toBeInstanceOf(BadRequestException);
  });
});
