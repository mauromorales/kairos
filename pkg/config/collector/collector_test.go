package collector_test

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	. "github.com/kairos-io/kairos/pkg/config/collector"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v1"
)

var _ = Describe("Config Collector", func() {
	Describe("Options", func() {
		var options *Options

		BeforeEach(func() {
			options = &Options{
				NoLogs: false,
			}
		})

		It("applies a defined option function", func() {
			option := func(o *Options) error {
				o.NoLogs = true
				return nil
			}

			Expect(options.NoLogs).To(BeFalse())
			Expect(options.Apply(option)).NotTo(HaveOccurred())
			Expect(options.NoLogs).To(BeTrue())
		})
	})
	Describe("MergeConfig", func() {
		var originalConfig, newConfig *Config
		BeforeEach(func() {
			originalConfig = &Config{}
			newConfig = &Config{}
		})

		Context("different keys", func() {
			BeforeEach(func() {
				err := yaml.Unmarshal([]byte("name: Mario"), originalConfig)
				Expect(err).ToNot(HaveOccurred())
				err = yaml.Unmarshal([]byte("surname: Bros"), newConfig)
				Expect(err).ToNot(HaveOccurred())
			})

			It("gets merged together", func() {
				Expect(originalConfig.MergeConfig(newConfig)).ToNot(HaveOccurred())
				surname, isString := (*originalConfig)["surname"].(string)
				Expect(isString).To(BeTrue())
				Expect(surname).To(Equal("Bros"))
			})
		})

		Context("same keys", func() {
			Context("when the key is a map", func() {
				BeforeEach(func() {
					err := yaml.Unmarshal([]byte(`---
info:
  name: Mario
`), originalConfig)
					Expect(err).ToNot(HaveOccurred())
					err = yaml.Unmarshal([]byte(`---
info:
  surname: Bros
`), newConfig)
					Expect(err).ToNot(HaveOccurred())
				})
				It("merges the keys", func() {
					Expect(originalConfig.MergeConfig(newConfig)).ToNot(HaveOccurred())
					info, isMap := (*originalConfig)["info"].(map[interface{}]interface{})
					Expect(isMap).To(BeTrue())
					Expect(info["name"]).To(Equal("Mario"))
					Expect(info["surname"]).To(Equal("Bros"))
					Expect(*originalConfig).To(HaveLen(1))
					Expect(info).To(HaveLen(2))
				})
			})

			Context("when the key is a string", func() {
				BeforeEach(func() {
					err := yaml.Unmarshal([]byte("name: Mario"), originalConfig)
					Expect(err).ToNot(HaveOccurred())
					err = yaml.Unmarshal([]byte("name: Luigi"), newConfig)
					Expect(err).ToNot(HaveOccurred())
				})

				It("overwrites", func() {
					Expect(originalConfig.MergeConfig(newConfig)).ToNot(HaveOccurred())
					name, isString := (*originalConfig)["name"].(string)
					Expect(isString).To(BeTrue())
					Expect(name).To(Equal("Luigi"))
					Expect(*originalConfig).To(HaveLen(1))
				})
			})
		})
	})

	Describe("MergeConfigURL", func() {
		var originalConfig *Config
		BeforeEach(func() {
			originalConfig = &Config{}
		})

		Context("when there is no config_url defined", func() {
			BeforeEach(func() {
				err := yaml.Unmarshal([]byte("name: Mario"), originalConfig)
				Expect(err).ToNot(HaveOccurred())
			})

			It("does nothing", func() {
				Expect(originalConfig.MergeConfigURL()).ToNot(HaveOccurred())
				Expect(*originalConfig).To(HaveLen(1))
			})
		})

		Context("when there is a chain of config_url defined", func() {
			var closeFunc ServerCloseFunc
			var port int
			var err error
			var tmpDir string
			var originalConfig *Config

			BeforeEach(func() {
				tmpDir, err = os.MkdirTemp("", "config_url_chain")
				Expect(err).ToNot(HaveOccurred())

				closeFunc, port, err = startAssetServer(tmpDir)
				Expect(err).ToNot(HaveOccurred())

				originalConfig = &Config{}
				err = yaml.Unmarshal([]byte(fmt.Sprintf(`---
config_url: http://127.0.0.1:%d/config1.yaml
name: Mario
surname: Bros
info:
  job: plumber
`, port)), originalConfig)
				Expect(err).ToNot(HaveOccurred())

				err := os.WriteFile(path.Join(tmpDir, "config1.yaml"), []byte(fmt.Sprintf(`
---
config_url: http://127.0.0.1:%d/config2.yaml
surname: Bras
`, port)), os.ModePerm)
				Expect(err).ToNot(HaveOccurred())

				err = os.WriteFile(path.Join(tmpDir, "config2.yaml"), []byte(`
---
info:
  girlfriend: princess
`), os.ModePerm)
				Expect(err).ToNot(HaveOccurred())
			})

			AfterEach(func() {
				closeFunc()
				err := os.RemoveAll(tmpDir)
				Expect(err).ToNot(HaveOccurred())
			})

			It("merges them all together", func() {
				err := originalConfig.MergeConfigURL()
				Expect(err).ToNot(HaveOccurred())

				name, ok := (*originalConfig)["name"].(string)
				Expect(ok).To(BeTrue())
				Expect(name).To(Equal("Mario"))

				surname, ok := (*originalConfig)["surname"].(string)
				Expect(ok).To(BeTrue())
				Expect(surname).To(Equal("Bras"))

				info, ok := (*originalConfig)["info"].(map[interface{}]interface{})
				Expect(ok).To(BeTrue())
				Expect(info["job"]).To(Equal("plumber"))
				Expect(info["girlfriend"]).To(Equal("princess"))

				Expect(*originalConfig).To(HaveLen(4))
			})
		})
	})

	Describe("Scan", func() {
		Context("multiple sources are defined", func() {
			var cmdLinePath, serverDir, tmpDir, tmpDir1, tmpDir2 string
			var err error
			var closeFunc ServerCloseFunc
			var port int

			BeforeEach(func() {
				// Prepare the cmdline config_url chain
				serverDir, err = os.MkdirTemp("", "config_url_chain")
				Expect(err).ToNot(HaveOccurred())
				closeFunc, port, err = startAssetServer(serverDir)
				Expect(err).ToNot(HaveOccurred())
				cmdLinePath = createRemoteConfigs(serverDir, port)

				tmpDir1, err = os.MkdirTemp("", "config1")
				Expect(err).ToNot(HaveOccurred())
				err := os.WriteFile(path.Join(tmpDir1, "local_config_1.yaml"), []byte(fmt.Sprintf(`
---
config_url: http://127.0.0.1:%d/remote_config_3.yaml
local_key_1: local_value_1
`, port)), os.ModePerm)
				Expect(err).ToNot(HaveOccurred())
				err = os.WriteFile(path.Join(serverDir, "remote_config_3.yaml"), []byte(fmt.Sprintf(`
---
config_url: http://127.0.0.1:%d/remote_config_4.yaml
remote_key_3: remote_value_3
`, port)), os.ModePerm)
				Expect(err).ToNot(HaveOccurred())

				err = os.WriteFile(path.Join(serverDir, "remote_config_4.yaml"), []byte(`
---
options:
  remote_option_1: remote_option_value_1
`), os.ModePerm)
				Expect(err).ToNot(HaveOccurred())

				tmpDir2, err = os.MkdirTemp("", "config2")
				Expect(err).ToNot(HaveOccurred())
				err = os.WriteFile(path.Join(tmpDir2, "local_config_2.yaml"), []byte(fmt.Sprintf(`
---
config_url: http://127.0.0.1:%d/remote_config_5.yaml
local_key_2: local_value_2
`, port)), os.ModePerm)
				Expect(err).ToNot(HaveOccurred())
				err = os.WriteFile(path.Join(serverDir, "remote_config_5.yaml"), []byte(fmt.Sprintf(`
---
config_url: http://127.0.0.1:%d/remote_config_6.yaml
remote_key_4: remote_value_4
`, port)), os.ModePerm)
				Expect(err).ToNot(HaveOccurred())

				err = os.WriteFile(path.Join(serverDir, "remote_config_6.yaml"), []byte(`
---
options:
  remote_option_2: remote_option_value_2
`), os.ModePerm)
				Expect(err).ToNot(HaveOccurred())
			})

			AfterEach(func() {
				err = os.RemoveAll(serverDir)
				Expect(err).ToNot(HaveOccurred())
				err = os.RemoveAll(tmpDir)
				Expect(err).ToNot(HaveOccurred())
				err = os.RemoveAll(tmpDir1)
				Expect(err).ToNot(HaveOccurred())
				err = os.RemoveAll(tmpDir2)
				Expect(err).ToNot(HaveOccurred())

				closeFunc()
			})

			It("merges all the sources accordingly", func() {
				c, err := Scan(MergeBootLine, WithBootCMDLineFile(cmdLinePath),
					Directories(tmpDir1, tmpDir2))
				Expect(err).ToNot(HaveOccurred())

				config_url, ok := (*c)["config_url"].(string)
				Expect(ok).To(BeTrue())
				Expect(config_url).To(MatchRegexp("remote_config_2.yaml"))

				k := (*c)["local_key_1"].(string)
				Expect(k).To(Equal("local_value_1"))
				k = (*c)["local_key_2"].(string)
				Expect(k).To(Equal("local_value_2"))
				k = (*c)["remote_key_1"].(string)
				Expect(k).To(Equal("remote_value_1"))
				k = (*c)["remote_key_2"].(string)
				Expect(k).To(Equal("remote_value_2"))
				k = (*c)["remote_key_3"].(string)
				Expect(k).To(Equal("remote_value_3"))
				k = (*c)["remote_key_4"].(string)
				Expect(k).To(Equal("remote_value_4"))

				options := (*c)["options"].(map[interface{}]interface{})
				Expect(options["foo"]).To(Equal("bar"))
				Expect(options["remote_option_1"]).To(Equal("remote_option_value_1"))
				Expect(options["remote_option_2"]).To(Equal("remote_option_value_2"))

				player := (*c)["player"].(map[interface{}]interface{})
				Expect(player["name"]).To(Equal("Dimitris"))
				Expect(player["surname"]).To(Equal("Bros"))
			})
		})
	})

	Describe("String", func() {
		var conf *Config
		BeforeEach(func() {
			conf = &Config{}
			err := yaml.Unmarshal([]byte("name: Mario"), conf)
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns the YAML string representation of the Config", func() {
			s, err := conf.String()
			Expect(err).ToNot(HaveOccurred())
			Expect(s).To(Equal(`#cloud-config

name: Mario
`), s)
		})
	})
})

func createRemoteConfigs(serverDir string, port int) string {
	err := os.WriteFile(path.Join(serverDir, "remote_config_1.yaml"), []byte(fmt.Sprintf(`
---
config_url: http://127.0.0.1:%d/remote_config_2.yaml
player:
  name: Dimitris
remote_key_1: remote_value_1
`, port)), os.ModePerm)
	Expect(err).ToNot(HaveOccurred())
	err = os.WriteFile(path.Join(serverDir, "remote_config_2.yaml"), []byte(`
---
player:
  surname: Bros
remote_key_2: remote_value_2
`), os.ModePerm)
	Expect(err).ToNot(HaveOccurred())

	cmdLinePath := filepath.Join(serverDir, "cmdline")
	// We put the cmdline in the same dir, it doesn't matter.
	cmdLine := fmt.Sprintf(`config_url="http://127.0.0.1:%d/remote_config_1.yaml" player.name="Mario" options.foo=bar`, port)
	err = os.WriteFile(cmdLinePath, []byte(cmdLine), os.ModePerm)
	Expect(err).ToNot(HaveOccurred())

	return cmdLinePath
}
