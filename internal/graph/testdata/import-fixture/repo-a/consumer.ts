import { STATUS } from '~/utils/enums';
import { helper } from './sibling';
import { createApp } from 'vue';

export function run() {
  return STATUS + helper() + String(createApp);
}
