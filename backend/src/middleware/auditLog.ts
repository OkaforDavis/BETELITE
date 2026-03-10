import { Request, Response, NextFunction } from 'express';
import prisma from '../config/database';

export const auditLog = (actionType: string) => {
  return async (req: Request, res: Response, next: NextFunction) => {
    const originalSend = res.send;

    res.send = function (data: any): Response {
      res.send = originalSend;

      if (res.statusCode < 400) {
        prisma.auditLog
          .create({
            data: {
              userId: req.user?.id || null,
              actionType,
              ipAddress: req.ip || req.socket.remoteAddress || 'unknown',
              userAgent: req.get('user-agent') || null,
              newValues: typeof data === 'string' ? JSON.parse(data) : data,
            },
          })
          .catch((err) => console.error('Audit log error:', err));
      }

      return originalSend.call(this, data);
    };

    next();
  };
};
