package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	DatabaseURL                  string `mapstructure:"DATABASE_URL"`
	Port                         string `mapstructure:"PORT"`
	// Azure Blob Storage — jika di-set, digunakan sebagai storage provider utama
	AzureStorageConnectionString string `mapstructure:"AZURE_STORAGE_CONNECTION_STRING"`
	AzureStorageContainerName    string `mapstructure:"AZURE_STORAGE_CONTAINER_NAME"`
	AzureStorageAccountName      string `mapstructure:"AZURE_STORAGE_ACCOUNT_NAME"`
}

func LoadConfig() *Config {
	viper.SetConfigFile(".env")
	viper.AutomaticEnv() // read from environment variables if set

	if err := viper.ReadInConfig(); err != nil {
		log.Println("No .env file found or error loading it, relying on environment variables")
	}

	// BindEnv WAJIB agar Unmarshal() membaca dari env vars container,
	// bukan hanya dari file .env.
	// Bug Viper: AutomaticEnv() hanya bekerja untuk viper.Get() langsung,
	// bukan saat Unmarshal dipanggil. Tanpa BindEnv, field struct tetap kosong
	// di lingkungan container meskipun env var sudah di-set.
	_ = viper.BindEnv("DATABASE_URL")
	_ = viper.BindEnv("PORT")
	_ = viper.BindEnv("AZURE_STORAGE_CONNECTION_STRING")
	_ = viper.BindEnv("AZURE_STORAGE_CONTAINER_NAME")
	_ = viper.BindEnv("AZURE_STORAGE_ACCOUNT_NAME")

	viper.SetDefault("PORT", "8080")
	viper.SetDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/transit_db?sslmode=disable")
	viper.SetDefault("AZURE_STORAGE_CONTAINER_NAME", "blobacacontainer")
	viper.SetDefault("AZURE_STORAGE_ACCOUNT_NAME", "blobacaghal")

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		log.Fatalf("Unable to decode into struct, %v", err)
	}

	return &config
}
