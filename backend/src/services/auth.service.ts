import bcrypt from 'bcrypt';
import jwt from 'jsonwebtoken';
import { v4 as uuidv4 } from 'uuid';
import prisma from '../config/database';
import { jwtConfig } from '../config/jwt';
import { ErrorResponse } from '../utils/errorResponse';

export class AuthService {
  async register(data: {
    username: string;
    email: string;
    password: string;
    dateOfBirth?: Date;
    countryCode?: string;
  }) {
    const existingUser = await prisma.user.findFirst({
      where: {
        OR: [{ email: data.email }, { username: data.username }],
      },
    });

    if (existingUser) {
      throw new ErrorResponse('User already exists', 400);
    }

    const hashedPassword = await bcrypt.hash(data.password, 12);

    const user = await prisma.$transaction(async (tx) => {
      const newUser = await tx.user.create({
        data: {
          username: data.username,
          email: data.email,
          passwordHash: hashedPassword,
          dateOfBirth: data.dateOfBirth,
          countryCode: data.countryCode,
          accountStatus: 'active',
          emailVerified: false,
        },
        select: {
          id: true,
          username: true,
          email: true,
          userType: true,
        },
      });

      await tx.userProfile.create({
        data: {
          userId: newUser.id,
          displayName: newUser.username,
        },
      });

      await tx.wallet.create({
        data: {
          userId: newUser.id,
          balanceAvailable: 0,
          balanceLocked: 0,
        },
      });

      return newUser;
    });

    const token = this.generateToken(user.id);

    return { user, token };
  }

  async login(email: string, password: string, ipAddress: string, userAgent: string) {
    const user = await prisma.user.findUnique({
      where: { email },
      select: {
        id: true,
        username: true,
        email: true,
        passwordHash: true,
        userType: true,
        accountStatus: true,
      },
    });

    if (!user) {
      await this.logSecurityEvent('failed_login', null, ipAddress, 'low', { email });
      throw new ErrorResponse('Invalid credentials', 401);
    }

    const isPasswordValid = await bcrypt.compare(password, user.passwordHash);

    if (!isPasswordValid) {
      await this.logSecurityEvent('failed_login', user.id, ipAddress, 'medium', { email });
      throw new ErrorResponse('Invalid credentials', 401);
    }

    if (user.accountStatus !== 'active') {
      throw new ErrorResponse('Account is not active', 403);
    }

    await prisma.user.update({
      where: { id: user.id },
      data: { lastLogin: new Date() },
    });

    const token = this.generateToken(user.id);
    const sessionId = uuidv4();

    await prisma.userSession.create({
      data: {
        sessionId,
        userId: user.id,
        ipAddress,
        userAgent,
        csrfToken: uuidv4(),
        expiresAt: new Date(Date.now() + 7 * 24 * 60 * 60 * 1000),
      },
    });

    return {
      user: {
        id: user.id,
        username: user.username,
        email: user.email,
        userType: user.userType,
      },
      token,
    };
  }

  async logout(userId: bigint, sessionId: string) {
    await prisma.userSession.deleteMany({
      where: {
        userId,
        sessionId,
      },
    });
  }

  private generateToken(userId: bigint): string {
    return jwt.sign({ id: userId.toString() }, jwtConfig.secret, {
      expiresIn: jwtConfig.expiresIn,
    });
  }

  private async logSecurityEvent(
    eventType: string,
    userId: bigint | null,
    ipAddress: string,
    severity: string,
    details: any
  ) {
    await prisma.securityEvent.create({
      data: {
        eventType: eventType as any,
        userId,
        ipAddress,
        severity: severity as any,
        details,
      },
    });
  }
}
