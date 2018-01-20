package main

import (
        "os"
        "io/ioutil"
        "time"
        "net"
        "encoding/binary"
        "encoding/hex"
        "encoding/json"
        "fmt"
        "log"
        "crypto/md5"
        "crypto/aes"
        "crypto/cipher"
        "github.com/andreburgaud/crypt2go/padding"
	"github.com/redhatzer0/huejack"
)

func key_iv(token [16]byte) ([16]byte,[16]byte) {
        // Key = md5(token) - iv = md5(md5(token) + token)
        md5sum := md5.Sum(token[:])
        concatted := append(md5sum[:], token[:]...)
        iv := md5.Sum(concatted)
        return md5sum, iv
}

func encrypt(plaintext []byte, token [16]byte) []byte {

        key, iv := key_iv(token)
        padder := padding.NewPkcs7Padding(16)

        padded, err := padder.Pad(plaintext)
        if err != nil {
                log.Fatal(err)
        }

        block, err := aes.NewCipher(key[:])
        if err != nil {
                log.Fatal(err)
        }

        mode := cipher.NewCBCEncrypter(block, iv[:])

        ciphertext := make([]byte, len(padded))
        mode.CryptBlocks(ciphertext, padded)

        return ciphertext
}



type device struct {
        ADDR    string
        TOKEN   string
        ID      int
}

type mrequest struct {
        ID              int `json:"id"`
        METHOD          string `json:"method"`
	PARAMS		[]interface{} `json:"params"`
}


func send(command string, params []interface{}, dev *device) {
        _send(command, params, dev, 3)
}

func _send(command string, params []interface{}, dev *device, retries int) {


        device_id, device_ts := discover(dev)

        var Request mrequest

        Request.ID = dev.ID
        dev.ID++
        Request.METHOD = command
        Request.PARAMS = params

        b, err := json.Marshal(Request)
        if err != nil {
                log.Fatal(err)
        }

        token_bytes, err := hex.DecodeString(dev.TOKEN)
        if err != nil {
                log.Fatal(err)
        }
        var token [16]byte
        copy(token[:], token_bytes[:])

        payload := encrypt(b, token)

        var buffer []byte
        magic := []byte{0x21, 0x31}
        unknown := make([]byte, 4)
        stamp := make([]byte, 4)
        checksum := make([]byte, 16)
        length := make([]byte, 2)

        dts := binary.BigEndian.Uint32(device_ts[:])
        binary.BigEndian.PutUint32(stamp, dts+1)


        binary.BigEndian.PutUint16(length, uint16(32 + len(payload)))

        buffer = append(buffer[:], magic[:]...)
        buffer = append(buffer[:], length[:]...)
        buffer = append(buffer[:], unknown[:]...)
        buffer = append(buffer[:], device_id[:]...)
        buffer = append(buffer[:], device_ts[:]...)
        buffer = append(buffer[:], checksum[:]...)

        buffer = append(buffer[:], payload[:]...)

        var md5input []byte
        md5input = append(md5input[:], buffer[0:16]...)
        md5input = append(md5input[:], token[:]...)
        md5input = append(md5input[:], payload[:]...)

        md5sum := md5.Sum(md5input)

        copy(buffer[16:32], md5sum[:])

        con, err := net.Dial("udp", dev.ADDR)
        if err != nil {
                log.Fatal(err)
        }

        _, err = con.Write(buffer)
        if err != nil {
                log.Fatal(err)
        }

        incoming := make([]byte, 1024)

        con.SetReadDeadline(time.Now().Add(1 * time.Second))
        read, err := con.Read(incoming)
        if err != nil {
                if retries > 0 {
                        log.Printf("Retrying ... %d times left\n", retries)
                        dev.ID = dev.ID + 100
                        _send(command, params, dev, retries - 1)
                } else {
                        log.Fatal(err)
                }
        } else {
                fmt.Printf("Read %d\n", read)
        }

}


func decrypt(ciphertext []byte, token [16]byte) []byte {
        key, iv := key_iv(token)
        padder := padding.NewPkcs7Padding(128)

        block, err := aes.NewCipher(key[:])
        if err != nil {
                log.Fatal(err)
        }

        mode := cipher.NewCBCDecrypter(block, iv[:])

        mode.CryptBlocks(ciphertext, ciphertext)

        unpadded, err := padder.Unpad(ciphertext)
        if err != nil {
                log.Fatal(err)
        }
        return unpadded
}


// Discover / Handshake
func discover(dev *device) ([4]byte, [4]byte) {
        const hellobytes = "21310020ffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
        decoded, err := hex.DecodeString(hellobytes)
        if err != nil {
                log.Fatal(err)
        }

        con, err := net.Dial("udp", dev.ADDR)
        if err != nil {
                log.Fatal(err)
        }

        _, err = con.Write(decoded)
        if err != nil {
                log.Fatal(err)
        }

        incoming := make([]byte, 256)

        _, err = con.Read(incoming)
        if err != nil {
                log.Fatal(err)
        }


        var did [4]byte
        copy(did[:], incoming[8:12])

        var dts [4]byte
        copy(dts[:], incoming[12:16])

        return did,dts
}

func get_local_token() string {
        token, err := ioutil.ReadFile("/mnt/data/miio/device.token")
        if err != nil {
                return ""
        }
        tokenstring := fmt.Sprintf("%X", token)
        return tokenstring
}



func get_device() device {

        var dev device

        data, err := ioutil.ReadFile("data.json")
        if err != nil {
                token := get_local_token()
                if len(token) == 0 {
                        log.Fatal("No config found and couldn't find local token")
                }
                dev = device{ADDR: "127.0.0.1:54321", TOKEN: token}
        } else {
                err = json.Unmarshal(data, &dev)
                if err != nil {
                        log.Fatal(err)
                }
        }
        return dev
}

func save_device(dev device) {
        //Update or Create data.json with updated seq number
        js, err := json.Marshal(dev)
        if err != nil {
                log.Fatal(err)
        }
        err = ioutil.WriteFile("data.json", js, os.ModePerm)
        if err != nil {
                log.Fatal(err)
        }
}


var lights = [...]string{
	"Rock",
}

func main() {




	huejack.Handle(lights[:], func(key, val int) {

		dev := get_device()
		if val == 0 {
			log.Printf("Stop")
			send("app_stop", nil, &dev)
			log.Printf("and go back to dock")
			send("app_charge", nil, &dev)
		} else {
			pp := make([]interface{}, 1)
			pp[0] = val * 100 / 256
			log.Printf("Set Fanspeed to %d", pp[0])
			send("set_custom_mode", pp, &dev)
			log.Printf("Start")
			send("app_start", nil, &dev)
		}
		save_device(dev)


	})


	log.Fatal(huejack.ListenAndServe())
}
