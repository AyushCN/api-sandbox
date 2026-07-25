import { NextResponse } from 'next/server';
import prisma from '@/lib/db';
import { z } from 'zod';

const updateEnvSchema = z.object({
  status: z.enum(['IDLE', 'BUILDING', 'RUNNING', 'STOPPED', 'FAILED']).optional(),
  publicUrl: z.string().optional(),
  containerId: z.string().optional(),
  port: z.number().int().optional(),
});

export async function GET(
  request: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;
    const environment = await prisma.environment.findUnique({
      where: { id },
      include: {
        logs: {
          orderBy: { timestamp: 'desc' },
          take: 50,
        },
        metrics: {
          orderBy: { timestamp: 'desc' },
          take: 20,
        },
      },
    });

    if (!environment) {
      return NextResponse.json({ error: 'Environment not found' }, { status: 404 });
    }

    return NextResponse.json(environment);
  } catch (error) {
    console.error(`Failed to fetch environment:`, error);
    return NextResponse.json({ error: 'Internal Server Error' }, { status: 500 });
  }
}

export async function PATCH(
  request: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;
    const body = await request.json();
    const parsed = updateEnvSchema.safeParse(body);

    if (!parsed.success) {
      return NextResponse.json({ error: 'Invalid payload', details: parsed.error.format() }, { status: 400 });
    }

    const environment = await prisma.environment.update({
      where: { id },
      data: parsed.data,
    });

    return NextResponse.json(environment);
  } catch (error) {
    console.error(`Failed to update environment:`, error);
    return NextResponse.json({ error: 'Internal Server Error' }, { status: 500 });
  }
}
