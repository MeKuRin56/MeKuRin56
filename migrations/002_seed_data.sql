-- 002_seed_data.sql
-- Seed Initial Rooms
INSERT INTO rooms (room_number, room_type, price, floor, status, description)
VALUES 
('101', 'Standard', 4500.00, 1, 'occupied', 'ห้องพักมาตรฐาน ชั้น 1 พร้อมเตียงและตู้เสื้อผ้า'),
('102', 'Standard', 4500.00, 1, 'vacant', 'ห้องพักมาตรฐาน ชั้น 1 วิวสวน'),
('103', 'Deluxe', 5500.00, 1, 'vacant', 'ห้องพักดีลักซ์ ชั้น 1 พร้อมระเบียงกว้าง'),
('201', 'Deluxe', 5500.00, 2, 'occupied', 'ห้องพักดีลักซ์ ชั้น 2 เครื่องปรับอากาศอินเวอร์เตอร์'),
('202', 'Studio', 6500.00, 2, 'vacant', 'ห้องสตูดิโอ ชั้น 2 ครัวขนาดเล็กพร้อมตู้เย็น'),
('301', 'Suite', 8000.00, 3, 'occupied', 'ห้องสวีท ชั้น 3 พื้นที่กว้างขวาง วิวเมือง'),
('302', 'Standard', 4800.00, 3, 'maintenance', 'ห้องพักมาตรฐาน ชั้น 3 กำลังปรับปรุงระบบปรับอากาศ')
ON CONFLICT (room_number) DO NOTHING;
