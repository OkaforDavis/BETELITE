import express, { Express, Request, Response, NextFunction } from 'express';
import cors from 'cors';
import helmet from 'helmet';
import compression from 'compression';
import { Server as SocketIOServer } from 'socket.io';
import http from 'http';
import dotenv from 'dotenv';

dotenv.config();

const app: Express = express();
const server = http.createServer(app);
const io = new SocketIOServer(server, {
    cors: {
        origin: process.env.FRONTEND_URL || 'http://localhost:3000',
        methods: ['GET', 'POST']
    }
});

// Middleware
app.use(helmet());
app.use(compression());
app.use(cors());
app.use(express.json({ limit: '10mb' }));
app.use(express.urlencoded({ limit: '10mb', extended: true }));

// Serve mobile app
app.use(express.static('public'));

// Health check
app.get('/health', (req: Request, res: Response) => {
    res.json({ status: 'ok', timestamp: new Date() });
});

// API Routes will be added here
app.get('/api/health', (req: Request, res: Response) => {
    res.json({ status: 'ok', service: 'betelite-api' });
});

// Socket.IO Events
const socketManager = require('./services/socketManager');

io.on('connection', (socket) => {
    console.log('User connected:', socket.id);

    // Join match room
    socket.on('join_match', (data) => {
        socketManager.handleJoinMatch(socket, data, io);
    });

    // Leave match room
    socket.on('leave_match', (data) => {
        socketManager.handleLeaveMatch(socket, data, io);
    });

    // Chat messages
    socket.on('chat_message', (data) => {
        socketManager.handleChatMessage(socket, data, io);
    });

    // Place bet
    socket.on('place_bet', (data) => {
        socketManager.handlePlaceBet(socket, data, io);
    });

    // Heartbeat
    socket.on('ping', () => {
        socket.emit('pong');
    });

    socket.on('disconnect', () => {
        console.log('User disconnected:', socket.id);
        socketManager.handleDisconnect(socket, io);
    });
});

// Error handling middleware
app.use((err: any, req: Request, res: Response, next: NextFunction) => {
    console.error('Error:', err);
    res.status(err.status || 500).json({
        error: err.message || 'Internal server error'
    });
});

// 404 handler
app.use((req: Request, res: Response) => {
    res.status(404).json({ error: 'Not found' });
});

const PORT = process.env.PORT || 3000;

server.listen(PORT, () => {
    console.log(`\n🚀 BETELITE Server running on port ${PORT}`);
    console.log(`📱 Mobile app: http://localhost:${PORT}/mobile/`);
    console.log(`🔗 API: http://localhost:${PORT}/api/`);
    console.log(`⚡ WebSocket: ws://localhost:${PORT}\n`);
});

export { app, server, io };
