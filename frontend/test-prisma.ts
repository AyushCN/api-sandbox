import prisma from './lib/db.js';

async function main() {
  const envs = await prisma.environment.findMany();
  console.log('Environments:', envs);
}

main().catch(console.error).finally(() => prisma.$disconnect());
