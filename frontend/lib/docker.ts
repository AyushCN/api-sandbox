import Docker from 'dockerode';
import { exec } from 'child_process';
import { promisify } from 'util';
import path from 'path';
import fs from 'fs/promises';
import crypto from 'crypto';
import prisma from './db';

const execAsync = promisify(exec);
const docker = new Docker({ socketPath: '/var/run/docker.sock' });

/**
 * Helper to pull a repository and build a Docker image.
 */
export async function cloneAndBuildImage(envId: string, gitUrl: string, branch: string = 'main') {
  const tmpDir = path.join(process.cwd(), 'tmp', envId);
  const imageTag = `api-sandbox-${envId.toLowerCase()}`;

  try {
    await prisma.log.create({
      data: {
        environmentId: envId,
        message: `Cloning repository ${gitUrl} (branch: ${branch})...`,
        level: 'info',
      }
    });

    // 1. Clone repository
    await execAsync(`git clone --depth 1 --branch ${branch} ${gitUrl} ${tmpDir}`);

    await prisma.log.create({
      data: {
        environmentId: envId,
        message: `Building Docker image ${imageTag}...`,
        level: 'info',
      }
    });

    // 2. Build Docker Image programmatically
    const stream = await docker.buildImage({
      context: tmpDir,
      src: ['.'], 
    }, { t: imageTag });

    await new Promise((resolve, reject) => {
      docker.modem.followProgress(
        stream,
        (err, res) => err ? reject(err) : resolve(res),
        async (progress) => {
          if (progress.stream) {
            const message = progress.stream.trim();
            if (message) {
              await prisma.log.create({
                data: {
                  environmentId: envId,
                  message,
                  level: 'info'
                }
              }).catch(console.error); // Do not crash build on log failure
            }
          }
        }
      );
    });

    await prisma.log.create({
      data: {
        environmentId: envId,
        message: `Image built successfully.`,
        level: 'info',
      }
    });

    return imageTag;
  } catch (error: any) {
    await prisma.log.create({
      data: {
        environmentId: envId,
        message: `Build failed: ${error.message || error}`,
        level: 'error',
      }
    });
    throw error;
  } finally {
    // 3. Cleanup temp directory
    try {
      await fs.rm(tmpDir, { recursive: true, force: true });
    } catch (e) {
      console.error(`Failed to cleanup temp dir ${tmpDir}:`, e);
    }
  }
}

/**
 * Helper to start a built image with resource limits and dynamic port mapping.
 */
export async function startContainer(envId: string, imageTag: string) {
  try {
    await prisma.log.create({
      data: {
        environmentId: envId,
        message: `Starting container for image ${imageTag}...`,
        level: 'info',
      }
    });

    const container = await docker.createContainer({
      Image: imageTag,
      name: `api-sandbox-env-${envId}`,
      HostConfig: {
        Memory: 512 * 1024 * 1024, // 512MB limit
        CpuShares: 512, // Half a core roughly
        PortBindings: {
          '3000/tcp': [{ HostPort: '0' }], // Assuming apps expose 3000 by default. Dynamically map host port.
          '8080/tcp': [{ HostPort: '0' }],
          '80/tcp': [{ HostPort: '0' }],
        },
        AutoRemove: false, // We'll handle removal to inspect exit codes if needed
      },
      ExposedPorts: {
        '3000/tcp': {},
        '8080/tcp': {},
        '80/tcp': {},
      }
    });

    await container.start();

    // Inspect container to get the assigned dynamic port
    const data = await container.inspect();
    const ports = data.NetworkSettings.Ports;
    
    // Find the first mapped port
    let assignedPort = null;
    for (const key in ports) {
      if (ports[key] && ports[key].length > 0) {
        assignedPort = parseInt(ports[key][0].HostPort, 10);
        break;
      }
    }

    if (!assignedPort) {
      throw new Error("Container started but no ports were mapped to the host.");
    }

    await prisma.log.create({
      data: {
        environmentId: envId,
        message: `Container started successfully on port ${assignedPort} (Container ID: ${container.id.substring(0, 12)}).`,
        level: 'info',
      }
    });

    return {
      containerId: container.id,
      port: assignedPort
    };
  } catch (error: any) {
    await prisma.log.create({
      data: {
        environmentId: envId,
        message: `Failed to start container: ${error.message || error}`,
        level: 'error',
      }
    });
    throw error;
  }
}

/**
 * Force stops and removes a container.
 */
export async function cleanupContainer(containerId: string) {
  try {
    const container = docker.getContainer(containerId);
    try {
      await container.stop();
    } catch (e: any) {
      // 304 means already stopped. Ignore.
      if (e.statusCode !== 304) {
         console.warn(`Container stop warning:`, e.message);
      }
    }
    await container.remove({ force: true });
    return true;
  } catch (error) {
    console.error(`Failed to cleanup container ${containerId}:`, error);
    return false;
  }
}

export default docker;
