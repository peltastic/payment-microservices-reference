import { Body, Controller, Post } from "@nestjs/common"
import { AuthorizeDto } from "./authorize-bank.dto"


@Controller('bank')
export class BankController {

  @Post('authorize')
  async authorize(@Body() body: AuthorizeDto) {
    await new Promise(r => setTimeout(r, Math.random() * 300))

    const declined = Math.random() < 0.2

    if (declined) {
      return {
        success: false,
        reference: `bank_ref_${Date.now()}`,
        code: 'card_declined',
        message: 'Card declined by issuer'
      }
    }

    return {
      success: true,
      reference: `bank_ref_${Date.now()}`,
      authorized_at: new Date().toISOString()
    }
  }

  @Post('void')
  void(@Body() body: { reference: string }) {
    return { success: true, voided: body.reference }
  }
}