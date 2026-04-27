@echo off
echo Starting BETELITE Test Environment...

echo Starting Node.js Backend...
start cmd /k "cd backend && npm install && npm run dev"

echo Starting Python AI Detection Service...
start cmd /k "cd detection_service && pip install -r requirements.txt && python app.py"

echo Both services are starting up. 
echo - Node.js Backend: http://localhost:3000
echo - Python Detection API: http://localhost:5000
echo - Mobile Frontend: http://localhost:3000/mobile

echo Starting Firebase Hosting Emulator...
start cmd /k "firebase serve --port 5005"
echo - Firebase Frontend: http://localhost:5005

