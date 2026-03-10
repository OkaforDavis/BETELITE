import { Request, Response, NextFunction } from 'express';
import jwt from 'jsonwebtoken';
import { jwtConfig } from '../config/jwt';
import { ErrorResponse } from '../utils/errorResponse';
import prisma from '../config/database';
import { UserType } from '@prisma/client';

export const protect = async (req: Request, res: Response, next: NextFunction) => {
  try {
    let token: string | undefined;

    if (req.headers.authorization?.startsWith('Bearer')) {
      token = req.headers.authorization.split(' ')[1];
    }

    if (!token) {
      throw new ErrorResponse('Not authorized to access this route', 401);
    }

    const decoded = jwt.verify(token, jwtConfig.secret) as { id: string };

    const user = await prisma.user.findUnique({
      where: { id: BigInt(decoded.id) },
      select: {
        id: true,
        username: true,
        email: true,
        userType: true,
        accountStatus: true,
      },
    });

    if (!user) {
      throw new ErrorResponse('User not found', 401);
    }

    if (user.accountStatus !== 'active') {
      throw new ErrorResponse('Account is not active', 403);
    }

    req.user = {
      id: user.id,
      username: user.username,
      email: user.email,
      userType: user.userType,
    };

    next();
  } catch (error: any) {
    next(new ErrorResponse('Not authorized', 401));
  }
};

export const restrictTo = (...roles: UserType[]) => {
  return (req: Request, res: Response, next: NextFunction) => {
    if (!req.user || !roles.includes(req.user.userType)) {
      return next(new ErrorResponse('You do not have permission to perform this action', 403));
    }
    next();
  };
};
