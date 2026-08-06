import buildQueue from './lib/queue.js';
import prisma from './lib/db.js';
import { cloneAndBuildImage, startContainer, cleanupContainer } from './lib/docker.js';

console.log('Background worker started. Listening for jobs...');

buildQueue.process(async (job) => {
  const { environmentId } = job.data;
  console.log(`Processing build job for environment ${environmentId}`);

  let environment = await prisma.environment.findUnique({ where: { id: environmentId } });
  if (!environment) {
    throw new Error(`Environment ${environmentId} not found`);
  }

  // Idempotency: Clean up any existing container for this environment if retrying
  if (environment.containerId) {
    console.log(`Cleaning up existing container ${environment.containerId} for retrying...`);
    await cleanupContainer(environment.containerId);
  }

  try {
    // 1. Build Image
    const imageTag = await cloneAndBuildImage(environment.id, environment.gitUrl, environment.githubBranch);

    // 2. Start Container
    const { containerId, port } = await startContainer(environment.id, imageTag);

    // 3. Update Database to RUNNING
    const publicUrl = `http://localhost:${port}`; // In production this would be a real domain
    await prisma.environment.update({
      where: { id: environment.id },
      data: {
        status: 'RUNNING',
        containerId,
        port,
        publicUrl
      }
    });

    console.log(`Environment ${environment.id} is now RUNNING on port ${port}`);

  } catch (error: any) {
    console.error(`Job failed for environment ${environmentId}:`, error);

    // Update status to FAILED
    await prisma.environment.update({
      where: { id: environmentId },
      data: { status: 'FAILED' }
    });

    throw error; // Will trigger Bull retry mechanism based on config
  }
});

// Handle graceful shutdown
process.on('SIGTERM', async () => {
  console.log('SIGTERM received. Shutting down worker...');
  await buildQueue.close();
  await prisma.$disconnect();
  process.exit(0);
});

process.on('SIGINT', async () => {
  console.log('SIGINT received. Shutting down worker...');
  await buildQueue.close();
  await prisma.$disconnect();
  process.exit(0);
});
