import {
  ArrayNotEmpty,
  IsArray,
  IsIn,
  IsOptional,
  IsString,
  IsUrl,
  MaxLength,
} from 'class-validator';

export const VALID_EVENTS = [
  'payment.succeeded',
  'payment.failed',
  'refund.succeeded',
  'refund.failed',
];

export class CreateEndpointDto {
  @IsUrl(
    { protocols: ['https'], require_protocol: true, require_tld: true },
    { message: 'url must be a valid HTTPS URL' },
  )
  url!: string;

  @IsArray()
  @ArrayNotEmpty()
  @IsString({ each: true })
  @IsIn(VALID_EVENTS, { each: true })
  events!: string[];

  @IsOptional()
  @IsString()
  @MaxLength(255)
  description?: string;
}
