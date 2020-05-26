package config

type Database struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Instance string `yaml:"name"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type TLS struct {
	PrivateKeyPath  string `yaml:"privateKeyPath"`
	CertificatePath string `yaml:"certificatePath"`
}

type Network struct {
	EntrypointPort           string `yaml:"entrypointPort"`
	EntrypointTLSPort        string `yaml:"entrypointTLSPort"`
	FullyQualifiedDomainName string `yaml:"fqdn"`
	TLS                      TLS    `yaml:"tls"`
	ExternalAccessIP         string `yaml:"externalAccessIP"`
}
type Log struct {
	Level    string      `yaml:"level"`
	Rotation LogRotation `yaml:"rotation"`
}

type LogRotation struct {
	MaxSizeMB  string `yaml:"maxSizeMB"`
	MaxAgeDays string `yaml:"maxAgeDays"`
}

type SingleNodeConfiguration struct {
	Database Database `yaml:"database"`
	Network  Network  `yaml:"network"`
	Log      Log      `yaml:"log"`
}
