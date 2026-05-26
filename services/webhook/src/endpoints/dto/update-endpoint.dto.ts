import { Type } from 'class-transformer';
import {
  ArrayNotEmpty,
  IsArray,
  IsBoolean,
  IsIn,
  IsOptional,
  IsString,
  IsUrl,
  MaxLength,
} from 'class-validator';
import { VALID_EVENTS } from './create-endpoint.dto';

export class UpdateEndpointDto {
  @IsOptional()
  @IsUrl(
    { protocols: ['https'], require_protocol: true, require_tld: true },
    { message: 'url must be a valid HTTPS URL' },
  )
  url?: string;

  @IsOptional()
  @IsArray()
  @ArrayNotEmpty()
  @IsString({ each: true })
  @IsIn(VALID_EVENTS, { each: true })
  events?: string[];

  @IsOptional()
  @IsString()
  @MaxLength(255)
  description?: string | null;

  @IsOptional()
  @Type(() => Boolean)
  @IsBoolean()
  isActive?: boolean;
}
