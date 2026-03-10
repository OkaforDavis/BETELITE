import prisma from '../config/database';
import { ErrorResponse } from '../utils/errorResponse';
import { Decimal } from '@prisma/client/runtime/library';

export class WalletService {
  async getBalance(userId: bigint) {
    const wallet = await prisma.wallet.findUnique({
      where: { userId },
      select: {
        balanceAvailable: true,
        balanceLocked: true,
        currencyCode: true,
        walletStatus: true,
      },
    });

    if (!wallet) {
      throw new ErrorResponse('Wallet not found', 404);
    }

    return wallet;
  }

  async deposit(userId: bigint, amount: number, paymentReference: string, paymentGateway: string) {
    if (amount <= 0) {
      throw new ErrorResponse('Invalid amount', 400);
    }

    return await prisma.$transaction(async (tx) => {
      const wallet = await tx.wallet.findUnique({
        where: { userId },
        select: { walletId: true, balanceAvailable: true, walletStatus: true },
      });

      if (!wallet) {
        throw new ErrorResponse('Wallet not found', 404);
      }

      if (wallet.walletStatus !== 'active') {
        throw new ErrorResponse('Wallet is not active', 403);
      }

      const balanceBefore = wallet.balanceAvailable;
      const balanceAfter = new Decimal(balanceBefore.toString()).plus(amount);

      await tx.wallet.update({
        where: { walletId: wallet.walletId },
        data: { balanceAvailable: balanceAfter.toNumber() },
      });

      const transaction = await tx.walletTransaction.create({
        data: {
          walletId: wallet.walletId,
          transactionType: 'deposit',
          amount,
          balanceBefore: balanceBefore.toNumber(),
          balanceAfter: balanceAfter.toNumber(),
          referenceType: 'payment',
          paymentReference,
          paymentGateway,
          transactionStatus: 'completed',
        },
      });

      return {
        transaction,
        newBalance: balanceAfter.toNumber(),
      };
    });
  }

  async withdraw(userId: bigint, amount: number) {
    if (amount <= 0) {
      throw new ErrorResponse('Invalid amount', 400);
    }

    return await prisma.$transaction(async (tx) => {
      const wallet = await tx.wallet.findUnique({
        where: { userId },
        select: { walletId: true, balanceAvailable: true, walletStatus: true },
      });

      if (!wallet) {
        throw new ErrorResponse('Wallet not found', 404);
      }

      if (wallet.walletStatus !== 'active') {
        throw new ErrorResponse('Wallet is not active', 403);
      }

      const balanceBefore = new Decimal(wallet.balanceAvailable.toString());
      
      if (balanceBefore.lessThan(amount)) {
        throw new ErrorResponse('Insufficient balance', 400);
      }

      const balanceAfter = balanceBefore.minus(amount);

      await tx.wallet.update({
        where: { walletId: wallet.walletId },
        data: { balanceAvailable: balanceAfter.toNumber() },
      });

      const transaction = await tx.walletTransaction.create({
        data: {
          walletId: wallet.walletId,
          transactionType: 'withdrawal',
          amount,
          balanceBefore: balanceBefore.toNumber(),
          balanceAfter: balanceAfter.toNumber(),
          referenceType: 'payment',
          transactionStatus: 'pending',
        },
      });

      return {
        transaction,
        newBalance: balanceAfter.toNumber(),
      };
    });
  }

  async lockFunds(userId: bigint, amount: number, referenceType: string, referenceId: bigint) {
    return await prisma.$transaction(async (tx) => {
      const wallet = await tx.wallet.findUnique({
        where: { userId },
        select: { walletId: true, balanceAvailable: true, balanceLocked: true },
      });

      if (!wallet) {
        throw new ErrorResponse('Wallet not found', 404);
      }

      const available = new Decimal(wallet.balanceAvailable.toString());
      
      if (available.lessThan(amount)) {
        throw new ErrorResponse('Insufficient balance', 400);
      }

      const newAvailable = available.minus(amount);
      const newLocked = new Decimal(wallet.balanceLocked.toString()).plus(amount);

      await tx.wallet.update({
        where: { walletId: wallet.walletId },
        data: {
          balanceAvailable: newAvailable.toNumber(),
          balanceLocked: newLocked.toNumber(),
        },
      });

      await tx.walletTransaction.create({
        data: {
          walletId: wallet.walletId,
          transactionType: referenceType === 'bet' ? 'bet_placed' : 'tournament_entry',
          amount,
          balanceBefore: available.toNumber(),
          balanceAfter: newAvailable.toNumber(),
          referenceType: referenceType as any,
          referenceId,
          transactionStatus: 'completed',
        },
      });

      return { newAvailable: newAvailable.toNumber(), newLocked: newLocked.toNumber() };
    });
  }

  async unlockFunds(userId: bigint, amount: number, transactionType: string, referenceId: bigint) {
    return await prisma.$transaction(async (tx) => {
      const wallet = await tx.wallet.findUnique({
        where: { userId },
        select: { walletId: true, balanceAvailable: true, balanceLocked: true },
      });

      if (!wallet) {
        throw new ErrorResponse('Wallet not found', 404);
      }

      const locked = new Decimal(wallet.balanceLocked.toString());
      const available = new Decimal(wallet.balanceAvailable.toString());

      if (locked.lessThan(amount)) {
        throw new ErrorResponse('Insufficient locked balance', 400);
      }

      const newLocked = locked.minus(amount);
      const newAvailable = available.plus(amount);

      await tx.wallet.update({
        where: { walletId: wallet.walletId },
        data: {
          balanceAvailable: newAvailable.toNumber(),
          balanceLocked: newLocked.toNumber(),
        },
      });

      await tx.walletTransaction.create({
        data: {
          walletId: wallet.walletId,
          transactionType: transactionType as any,
          amount,
          balanceBefore: available.toNumber(),
          balanceAfter: newAvailable.toNumber(),
          referenceType: 'bet',
          referenceId,
          transactionStatus: 'completed',
        },
      });

      return { newAvailable: newAvailable.toNumber(), newLocked: newLocked.toNumber() };
    });
  }

  async getTransactions(userId: bigint, limit: number = 50, offset: number = 0) {
    const wallet = await prisma.wallet.findUnique({
      where: { userId },
      select: { walletId: true },
    });

    if (!wallet) {
      throw new ErrorResponse('Wallet not found', 404);
    }

    const transactions = await prisma.walletTransaction.findMany({
      where: { walletId: wallet.walletId },
      orderBy: { createdAt: 'desc' },
      take: limit,
      skip: offset,
    });

    return transactions;
  }
}
