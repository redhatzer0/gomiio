# gomiio
Control Xiaomi vacuum cleaner with Alexa

This emulates a Hue bridge, with the vacuum as an emulated light. You can turn the vacuum on/off. By adjusting the brightness of the emulated light, you'll change the fanspeed on the vacuum

If you crosscompile it for linux/arm ( *GOOS=linux GOARCH=arm go build* ), you can copy the binary to the robot and run it directly there. (In this case, it reads the needed device token directly from the filesystem)

You can also run this on a different computer, as long as it's able to connect directly to the robot. In this case, you need to create a file called "data.json" in the same dir as the binary. The content should look like:

    {"ADDR":"192.168.1.50:54321","TOKEN":"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"}

(Change ADDR to the IP of your vacuum and change token to your device token)

After starting the program, you need to search for new smart home devices with alexa. It should find a new light called "Rock" which is the vacuum. 
