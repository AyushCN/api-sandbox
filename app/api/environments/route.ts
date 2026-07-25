import { NextResponse } from 'next/server';
import prisma from '@/lib/db';
import buildQueue from '@/lib/queue';
import { z } from 'zod';

const createEnvSchema = z.object({
  name: z.string().min(1),
  gitUrl: z.string().url(),
  githubBranch: z.string().default('main'),
});

export async function GET() {
  try {
    const environments = await prisma.environment.findMany({
      orderBy: { createdAt: 'desc' },
    });
    return NextResponse.json(environments);
  } catch (error) {
    console.error('Failed to fetch environments:', error);
    return NextResponse.json({ error: 'Internal Server Error' }, { status: 500 });
  }
}

export async function POST(request: Request) {
  try {
    const body = await request.json();
    const parsed = createEnvSchema.safeParse(body);
    
    if (!parsed.success) {
      return NextResponse.json({ error: 'Invalid payload', details: parsed.error.format() }, { status: 400 });
    }

    const { name, gitUrl, githubBranch } = parsed.data;

    const environment = await prisma.environment.create({
      data: {
        name,
        gitUrl,
        githubBranch,
        status: 'BUILDING', // Using enum value mapped to string
      },
    });

    await buildQueue.add({ environmentId: environment.id });

    return NextResponse.json(environment, { status: 201 });
  } catch (error) {
    console.error('Failed to create environment:', error);
    return NextResponse.json({ error: 'Internal Server Error' }, { status: 500 });
  }
}
