```
	DefaultGenesisJson = `{
    "nettopology":{
    },
    "alloc":{},
    "mstate":{
		"BroadcastInterval" :{         \\�㲥���ڼ��
			"LastBCNumber" : 0,        \\��������
			"LastReelectNumber" : 0,   \\��������
			"BCInterval" : 100,        \\�㲥����
			"BackupEnableNumber" : 0,  \\��������ʹ��ʱ��
			"BackupBCInterval" : 0     \\���������޸ĺ�Ĺ㲥����
		},
		"VIPCfg": [
					{
				"MinMoney": 0,         \\��������
				"InterestRate": 5,     \\һ���㲥����Ĭ��100�飬6s��������ʣ�������=��1+5*36/10000000)^(600*24*365/3600)=0.040
				"ElectUserNum": 0,     \\��������
				"StockScale": 1000     \\���㶨��ֵ
			},
			{
				"MinMoney": 1000000,    \\vip2����С��
				"InterestRate": 10,     \\һ���㲥����Ĭ��100�飬6s��������ʣ�������=��1+10*36/10000000)^(600*24*365/3600)=0.082
				"ElectUserNum": 3,      \\vip2����
				"StockScale": 1000      \\���㶨��ֵ
			},
		{
				"MinMoney": 10000000,    \\vip1����С��
				"InterestRate": 15,      \\һ���㲥����Ĭ��100�飬6s��������ʣ�������=��1+15*36/10000000)^(600*24*365/3600)=0.126
				"ElectUserNum": 5,       \\vip1����
				"StockScale": 1000       \\���㶨��ֵ
			}
		],
        "BlkCalcCfg":"1",                \\�̶����齱���㷨����
        "TxsCalcCfg":"1",                \\���׽����㷨����
        "InterestCalcCfg":"1",           \\��Ϣ�����㷨����
        "LotteryCalcCfg":"1",		     \\��Ʊ�����㷨����
        "SlashCalcCfg":"1",              \\�ͷ��㷨����
		"BlkRewardCfg": {
			"MinerMount": 3,             \\�󹤽�������man
			"MinerHalf": 5000000,        \\���۰�����
			"ValidatorMount": 7,         \\��֤�߽�������man
			"ValidatorHalf": 5000000,    \\��֤���۰�����
			"RewardRate": {
				"MinerOutRate": 4000,      \\����󹤽���10000��λ
				"ElectedMinerRate": 5000,  \\ѡ�п󹤽���10000��λ
				"FoundationMinerRate": 1000,\\����ά��10000��λ
				"LeaderRate": 4000,         \\��֤��leader����10000��λ
				"ElectedValidatorsRate": 5000, \\������֤�߽���10000��λ
				"FoundationValidatorRate": 1000, \\�������֤�߽���10000��λ
				"OriginElectOfflineRate": 5000,  \\��ѡ������֤�߽���10000��λ
				"BackupRewardRate": 5000         \\��ѡ��֤�߽���10000��λ
			}
		},
		"TxsRewardCfg": {
			"MinersRate": 0,                   \\�󹤽���
			"ValidatorsRate": 10000,           \\��֤�߽���
			"RewardRate": {
				"MinerOutRate": 4000,           \\����󹤽���10000��λ
				"ElectedMinerRate": 6000,       \\ѡ�п󹤽���10000��λ
				"FoundationMinerRate":0,        \\����ά��10000��λ
				"LeaderRate": 4000,             \\��֤��leader����10000��λ
				"ElectedValidatorsRate": 6000,  \\������֤�߽���10000��λ
				"FoundationValidatorRate": 0,    \\�������֤�߽���10000��λ
				"OriginElectOfflineRate": 5000,   \\��ѡ������֤�߽���10000��λ
				"BackupRewardRate": 5000          \\��ѡ��֤�߽���10000��λ
			}
		},
		"LotteryCfg": {
//			"LotteryCalc": "1",            
			"LotteryInfo": [{                      
				"PrizeLevel": 0,             \\��Ʊ�ļ���
				"PrizeNum": 1,               \\��Ʊ��ǰ�������Ŀ
				"PrizeMoney": 6               \\ÿ���н��˻��Ľ���
			}]
		},
		"InterestCfg": {
			"CalcInterval": 100,     \\��Ϣ�����ļ�Ϣ����
			"PayInterval": 3600      \\��Ϣ�����Ľ�Ϣ�������
		},
		"LeaderCfg": {
			"ParentMiningTime": 20,   
			"PosOutTime": 20,
			"ReelectOutTime": 40,
			"ReelectHandleInterval": 3
		},
		"SlashCfg": {
			"SlashRate": 7500       \\��Ϣ�ͷ�������10000��λ
		},
		"EleTime": {
			"MinerGen": 6,           \\��ѡ��ʱ��ʹ�õĵ�Ѻ��Ϣ������ڹ㲥���ڵĲ��
			"MinerNetChange": 5,     \\����ڹ㲥�������ɿ�����ͼ�Ĳ��
			"ValidatorGen": 4,       \\��֤��ѡ��ʱ��ʹ�õĵ�Ѻ��Ϣ������ڹ㲥���ڵĲ��
			"ValidatorNetChange": 3, \\����ڹ㲥����������֤������ͼ�Ĳ��
 			"VoteBeforeTime": 7      \\����ڹ㲥���ڹ�˽Կ���׵���ǰʱ��
		},
		"EleInfo": {
			"ValidatorNum": 19,       \\��֤����Ŀ
			"BackValidator": 5,       \\������֤����Ŀ
			"ElectPlug": "layerd"     \\ѡ���㷨�������
		},
		"ElectMinerNum": {
			"MinerNum": 21            \\����Ŀ
		},
		"ElectBlackList": null,       \\������
		"ElectWhiteListSwitcherCfg":  { \\ѡ�ٰ�����ʹ�ܿ���
			"Switcher" : false        \\�ر�
		},
		"ElectWhiteList": null        
    },
  "config": {
					"chainID": 1,           \\��id
					"byzantiumBlock": 0,     \\
					"homesteadBlock": 0,
					"eip155Block": 0,
			        "eip158Block": 0                        				             
	},
    "versionSignatures": [],                    \\�汾ǩ��
    "difficulty":"0x100",                   \\�����Ѷ�
    "timestamp":"0x5c26f140",                 \\����ʱ��
	"version": "1.0.0.0",            \\�汾��
  
	"signatures": [	],                        \\��ʼ��ǩ��
    "coinbase": "MAN.1111111111111111111cs", \\��ʼ�齱����
    "leader":"MAN.1111111111111111111cs", \\������leader
    "gasLimit": "0x2FEFD8",                     \\�����齻�׷�����
    "nonce": "0x00000000000000178",              \\���������
    "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
    "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
	"extraData": "0x68656c6c6f2c77656c636f6d6520746f206d617472697820776f726c6421"   \\�����鸽������"hello,welcome to matrix world!"
}
```