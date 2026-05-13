
import cv2
import numpy as np
img = np.zeros((200, 400, 3), dtype=np.uint8)
cv2.putText(img, 'SQUANTAKAY KILLS 19', (50, 100), cv2.FONT_HERSHEY_SIMPLEX, 1, (255, 255, 255), 2)
cv2.imwrite('test_ocr_img.jpg', img)
