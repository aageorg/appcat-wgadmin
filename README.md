# Wgadmin
This software is a Telegrm bot who provides Wireguard VPN settings to users. 

## Requirements
+ Bolt (https://github.com/boltdb/bolt/)
+ WgRPC (https://github.com/aageorg/wgrpc)
+ Accessible domain name with valid SSL certificate

## Getting started
1. Create your own telegram bot via @BotFather
2. Configure securely connection between wireguard server's side and wgadmin bot unless they are not on the same server. Use VPN or closed from the Internet local area network
3. Copy default.conf to appcat.conf Read instructions inside and edit file using your credentials and preferences. 
4. Launch your wgadmin bot
