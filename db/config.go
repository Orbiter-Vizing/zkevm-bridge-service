package db

// Config struct
type Config struct {
	// Database type
	Database string `mapstructure:"Database"`

	// Database name
	Name string `mapstructure:"Name"`

	// User name
	User string `mapstructure:"User"`

	// Password of the user
	Password string `mapstructure:"Password"`

	// Host address
	Host string `mapstructure:"Host"`

	// Port Number
	Port string `mapstructure:"Port"`

	// EnableLog
	EnableLog bool `mapstructure:"EnableLog"`

	LogFile string `mapstructure:"LogFile"`

	LogLevel string `mapstructure:"LogLevel" jsonschema:"enum=debug,enum=info,enum=warn,enum=error"`

	// Unit in milliseconds
	LogSlowTime int64 `mapstructure:"LogSlowTime" jsonschema:"enum=debug,enum=info,enum=warn,enum=error"`

	// MaxConns is the maximum number of connections in the pool.
	MaxConns int `mapstructure:"MaxConns"`
}
