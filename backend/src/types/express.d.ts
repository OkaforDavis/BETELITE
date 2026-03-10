import { UserType } from '@prisma/client';

declare global {
  namespace Express {
    interface Request {
      user?: {
        id: bigint;
        username: string;
        email: string;
        userType: UserType;
      };
    }
  }
}
