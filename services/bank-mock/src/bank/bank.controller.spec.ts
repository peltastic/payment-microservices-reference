import { Test, TestingModule } from '@nestjs/testing';
import { BankController } from './bank.controller';
import { BankService } from './bank.service';

describe('BankController', () => {
  let controller: BankController;

  beforeEach(async () => {
    const module: TestingModule = await Test.createTestingModule({
      controllers: [BankController],
      providers: [BankService],
    }).compile();

    controller = module.get<BankController>(BankController);
  });

  it('should be defined', () => {
    expect(controller).toBeDefined();
  });

  it('given bank authorization succeeds when authorize is called then returns an approval reference', async () => {
    // Arrange
    jest.spyOn(Math, 'random').mockReturnValue(0.9);
    jest.spyOn(Date, 'now').mockReturnValue(1700000000000);

    // Act
    const result = await controller.authorize({});

    // Assert
    expect(result).toMatchObject({
      success: true,
      reference: 'bank_ref_1700000000000',
    });
    jest.restoreAllMocks();
  });

  it('given bank authorization declines when authorize is called then returns a decline code', async () => {
    // Arrange
    jest.spyOn(Math, 'random').mockReturnValue(0.1);
    jest.spyOn(Date, 'now').mockReturnValue(1700000000001);

    // Act
    const result = await controller.authorize({});

    // Assert
    expect(result).toEqual({
      success: false,
      reference: 'bank_ref_1700000000001',
      code: 'card_declined',
      message: 'Card declined by issuer',
    });
    jest.restoreAllMocks();
  });
});
