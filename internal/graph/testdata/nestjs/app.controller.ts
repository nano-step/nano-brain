import { Controller, Get } from '@nestjs/common';

@Controller()
export class AppController {
  @Get()
  root() {
    return 'Hello World';
  }

  @Get('health')
  health() {
    return { status: 'ok' };
  }
}
