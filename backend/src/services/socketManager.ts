import { Socket } from 'socket.io';

interface MatchData {
    matchId: string;
    game: string;
    userId: string;
    username: string;
    betAmount?: number;
    prediction?: string;
}

class SocketManager {
    private matchRooms = new Map<string, Set<string>>();
    private userMatches = new Map<string, string>();
    private matchChats = new Map<string, Array<{ username: string; message: string; userId: string; timestamp: Date }>>();

    handleJoinMatch(socket: Socket, data: { matchId: string }, io: any) {
        const { matchId } = data;
        const userId = socket.handshake.query.userId;

        socket.join(`match:${matchId}`);
        this.userMatches.set(socket.id, matchId);

        if (!this.matchRooms.has(matchId)) {
            this.matchRooms.set(matchId, new Set());
        }
        this.matchRooms.get(matchId)!.add(socket.id);

        // Notify others
        io.to(`match:${matchId}`).emit('user_joined', {
            userId,
            spectators: this.matchRooms.get(matchId)!.size
        });

        console.log(`User joined match ${matchId}. Total in room: ${this.matchRooms.get(matchId)!.size}`);
    }

    handleLeaveMatch(socket: Socket, data: { matchId: string }, io: any) {
        const { matchId } = data;

        socket.leave(`match:${matchId}`);
        this.matchRooms.get(matchId)?.delete(socket.id);
        this.userMatches.delete(socket.id);

        io.to(`match:${matchId}`).emit('user_left', {
            spectators: this.matchRooms.get(matchId)?.size || 0
        });

        console.log(`User left match ${matchId}`);
    }

    handleChatMessage(socket: Socket, data: { matchId: string; message: string }, io: any) {
        const { matchId, message } = data;
        const userId = socket.handshake.query.userId as string;
        const username = socket.handshake.query.username as string;

        // Sanitize message
        const sanitized = message.trim().substring(0, 150);

        if (!sanitized) return;

        // Store in chat history
        if (!this.matchChats.has(matchId)) {
            this.matchChats.set(matchId, []);
        }

        const chatRecord = {
            username,
            message: sanitized,
            userId,
            timestamp: new Date()
        };

        const chatHistory = this.matchChats.get(matchId)!;
        chatHistory.push(chatRecord);

        // Keep last 100 messages
        if (chatHistory.length > 100) {
            chatHistory.shift();
        }

        // Broadcast to match room
        io.to(`match:${matchId}`).emit('chat_message', {
            username,
            message: sanitized,
            userId,
            timestamp: chatRecord.timestamp
        });

        console.log(`Chat in match ${matchId}: ${username}: ${sanitized}`);
    }

    handlePlaceBet(socket: Socket, data: MatchData, io: any) {
        const { matchId, betAmount, prediction } = data;
        const userId = socket.handshake.query.userId as string;
        const username = socket.handshake.query.username as string;

        // TODO: Process bet in database
        // - Deduct amount from user wallet
        // - Create bet record
        // - Create betting pool if not exists

        io.to(`match:${matchId}`).emit('bet_placed', {
            username,
            amount: betAmount,
            prediction,
            timestamp: new Date()
        });

        console.log(`Bet placed: ${username} - ₦${betAmount} on ${prediction}`);
    }

    handleDisconnect(socket: Socket, io: any) {
        const matchId = this.userMatches.get(socket.id);

        if (matchId) {
            this.matchRooms.get(matchId)?.delete(socket.id);
            io.to(`match:${matchId}`).emit('user_left', {
                spectators: this.matchRooms.get(matchId)?.size || 0
            });
        }

        this.userMatches.delete(socket.id);
    }

    // Get chat history for a match
    getChatHistory(matchId: string) {
        return this.matchChats.get(matchId) || [];
    }

    // Get spectator count for a match
    getSpectatorCount(matchId: string) {
        return this.matchRooms.get(matchId)?.size || 0;
    }
}

module.exports = new SocketManager();
