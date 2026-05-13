const fs = require('fs');

async function testOCR() {
    try {
        const imgPath = 'squanta_cod_score.jpg'; 
        if (!fs.existsSync(imgPath)) {
            console.log('Image not found. Creating a dummy text image...');
            const { execSync } = require('child_process');
            const pyScript = `
import cv2
import numpy as np
img = np.zeros((200, 400, 3), dtype=np.uint8)
cv2.putText(img, 'SQUANTAKAY KILLS 19', (50, 100), cv2.FONT_HERSHEY_SIMPLEX, 1, (255, 255, 255), 2)
cv2.imwrite('test_ocr_img.jpg', img)
`;
            fs.writeFileSync('create_test_img.py', pyScript);
            execSync('python create_test_img.py');
        }

        const fileData = fs.readFileSync(fs.existsSync(imgPath) ? imgPath : 'test_ocr_img.jpg');
        const b64 = fileData.toString('base64');
        const image_b64 = 'data:image/jpeg;base64,' + b64;

        const formData = new URLSearchParams();
        formData.append('game', 'COD Mobile');
        formData.append('target_gamertag', 'SQUANTAKAY');
        formData.append('image_b64', image_b64);

        const response = await fetch('http://localhost:3000/api/detect/frame', {
            method: 'POST',
            body: formData
        });

        const data = await response.json();
        console.log('OCR Response:', JSON.stringify(data, null, 2));
    } catch (err) {
        console.error('OCR Error:', err);
    }
}

testOCR();
