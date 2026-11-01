package dns

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

func addAddyRecords(scope constructs.Construct, zone awsroute53.IHostedZone) {
	awsroute53.NewMxRecord(scope, jsii.String("AddyMX"), &awsroute53.MxRecordProps{
		Zone: zone,
		Ttl:  awscdk.Duration_Seconds(jsii.Number(300)),
		Values: &[]*awsroute53.MxRecordValue{
			{HostName: jsii.String("mail.anonaddy.me"), Priority: jsii.Number(10)},
			{HostName: jsii.String("mail2.anonaddy.me"), Priority: jsii.Number(20)},
		},
	})
	awsroute53.NewTxtRecord(scope, jsii.String("AddySPF"), &awsroute53.TxtRecordProps{
		Zone: zone,
		Ttl:  awscdk.Duration_Seconds(jsii.Number(300)),
		Values: jsii.Strings(
			"v=spf1 include:spf.anonaddy.me -all",
			"aa-verify=183b894cd3b8313ce5c8d0e8ee9ffe3f9e887b41",
		),
	})
	for _, selector := range []string{"dk1", "dk2"} {
		awsroute53.NewCnameRecord(scope, jsii.String("AddyDKIM-"+selector), &awsroute53.CnameRecordProps{
			Zone:       zone,
			Ttl:        awscdk.Duration_Seconds(jsii.Number(300)),
			RecordName: jsii.String(selector + "._domainkey"),
			DomainName: jsii.String(selector + "._domainkey.anonaddy.me"),
		})
	}
	awsroute53.NewTxtRecord(scope, jsii.String("AddyDMARC"), &awsroute53.TxtRecordProps{
		Zone:       zone,
		Ttl:        awscdk.Duration_Seconds(jsii.Number(300)),
		RecordName: jsii.String("_dmarc"),
		Values:     jsii.Strings("v=DMARC1; p=quarantine; adkim=s"),
	})
}
