package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"strings"
	"time"
)

// adsadapter message : 0x02 - body - 0x03
const (
	StartChar = byte(0x02)
	EndChar   = byte(0x03)
)

func main() {
	var adsaAddr, xmlFile, statusTransacionID, deleteTransacionID, tmStatusTrID, tmOffTrID, tmOffChID string
	flag.StringVar(&adsaAddr, "addr", "127.0.0.1:8081", "ADSAdapter listen address")
	flag.StringVar(&xmlFile, "xml", "", "xml file to send")
	flag.StringVar(&statusTransacionID, "ss-status", "", "send StreamSegmentStatus with [transactionID]")
	flag.StringVar(&deleteTransacionID, "ss-delete", "", "send StreamSegmentDelete with [transactionID]")
	flag.StringVar(&tmStatusTrID, "tm-status", "", "send TimeMachineStatus with [transactionID]")
	flag.StringVar(&tmOffTrID, "tm-off-transaction", "", "send TimeMachineOff with [transactionID]")
	flag.StringVar(&tmOffChID, "tm-off-channel", "", "send TimeMachineOff with [channelID]")

	flag.Parse()

	if len(xmlFile) > 0 {
		buf, err := ioutil.ReadFile(xmlFile)
		if err != nil {
			log.Fatal(err)
		}
		_, err = sendXML(adsaAddr, string(buf[:]))
		if err != nil {
			log.Fatal(err)
		}
	} else if len(statusTransacionID) > 0 {
		_, err := streamSegmentStatus(adsaAddr, statusTransacionID)
		if err != nil {
			log.Fatal(err)
		}
	} else if len(deleteTransacionID) > 0 {
		_, err := streamSegmentDelete(adsaAddr, deleteTransacionID)
		if err != nil {
			log.Fatal(err)
		}
	} else if len(tmStatusTrID) > 0 {
		_, err := timeMachineStatus(adsaAddr, tmStatusTrID)
		if err != nil {
			log.Fatal(err)
		}
	} else if len(tmOffTrID) > 0 && len(tmOffChID) > 0 {
		_, err := timeMachineOff(adsaAddr, tmOffTrID, tmOffChID)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func sendXML(addr, xml string) (string, error) {
	beg := time.Now()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return "", fmt.Errorf("failed to dial %v, %v", addr, err)
	}

	_, err = conn.Write([]byte(xml))
	if err != nil {
		return "", fmt.Errorf("failed to write body to server, %v", err)
	}

	_, err = conn.Write([]byte{EndChar})
	if err != nil {
		return "", fmt.Errorf("failed to write end char to server, %v", err)
	}

	log.Printf("sended :\n")
	log.Printf("%s\n", xml)

	reply, err := bufio.NewReader(conn).ReadSlice(EndChar)
	if err != nil {
		return "", fmt.Errorf("failed to read reply from server, %v", err)
	}
	res := string(reply[1 : len(reply)-1])

	log.Printf("elapsed: %v\n", time.Since(beg))
	log.Printf("received:\n")
	log.Printf("%v\n", res)

	return res, nil
}

func streamSegmentStatus(addr, trid string) (reply string, err error) {
	xml := `<?xml version="1.0" encoding="utf-8" standalone="no" ?> 
<eADS>
	<Job>
		<Job_Name>stream segment status</Job_Name> 
		<Command_Date>DATETIME</Command_Date> 
		<Command Type="StreamSegmentStatus">
			<Command_Data Name="Transaction_ID" Value="TRANSACTIONID"/> 	
		</Command>
	</Job>
</eADS>
`
	xml = strings.Replace(xml, "TRANSACTIONID", trid, -1)
	xml = strings.Replace(xml, "DATETIME", timeToStr(time.Now()), -1)
	return sendXML(addr, xml)
}

func streamSegmentDelete(addr, trid string) (reply string, err error) {
	xml := `<?xml version="1.0" encoding="utf-8" standalone="no" ?> 
<eADS>
	<Job>
		<Job_Name>stream segment delete</Job_Name> 
		<Command_Date>DATETIME</Command_Date> 
		<Command Type="StreamSegmentDelete">
			<Command_Data Name="Transaction_ID" Value="TRANSACTIONID"/> 	
		</Command>
	</Job>
</eADS>
`
	xml = strings.Replace(xml, "TRANSACTIONID", trid, -1)
	xml = strings.Replace(xml, "DATETIME", timeToStr(time.Now()), -1)
	return sendXML(addr, xml)
}

func timeToStr(t time.Time) string {
	var layout = "2006-01-02 15:04:05"
	return t.Format(layout)
}

func timeMachineStatus(addr, trid string) (reply string, err error) {
	xml := `<?xml version="1.0" encoding="EUC-KR" standalone="no" ?> 
	<eADS>
		<Job>
			<Job_Name>time machine status</Job_Name>
			<Command_Date>DATETIME</Command_Date>
			<Command Type="TimeMachineStatus">
				<Command_Data Name="Transaction_ID" Value="TRANSACTIONID"/>
			</Command>
		</Job>
	</eADS>
`
	xml = strings.Replace(xml, "TRANSACTIONID", trid, -1)
	xml = strings.Replace(xml, "DATETIME", timeToStr(time.Now()), -1)
	return sendXML(addr, xml)
}

func timeMachineOff(addr, trid, chid string) (reply string, err error) {
	xml := `<?xml version="1.0" encoding="EUC-KR" standalone="no" ?> 
<eADS>
	<Job>
		<Job_Name>time machine off</Job_Name>
		<Command_Date>DATETIME</Command_Date>
		<Command Type="TimeMachineOff">
			<Command_Data Name="Channel_ID" Value="CHANNELID"/>
			<Command_Data Name="Transaction_ID" Value="TRANSACTIONID"/>
		</Command>
	</Job>
</eADS>
`
	xml = strings.Replace(xml, "CHANNELID", chid, -1)
	xml = strings.Replace(xml, "TRANSACTIONID", trid, -1)
	xml = strings.Replace(xml, "DATETIME", timeToStr(time.Now()), -1)
	return sendXML(addr, xml)
}
