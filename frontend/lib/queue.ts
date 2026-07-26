import Queue from 'bull';
import redis from './redis';

const buildQueue = new Queue('build-environments', {
  redis: process.env.REDIS_URL || 'redis://localhost:6379',
  defaultJobOptions: {
    attempts: 3,
    backoff: {
      type: 'exponential',
      delay: 2000,
    },
    removeOnComplete: true,
  }
});

export default buildQueue;
