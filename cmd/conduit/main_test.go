package main

import (
	_ "embed"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/algorand/indexer/conduit"
	"github.com/algorand/indexer/conduit/pipeline"
)

func TestBanner(t *testing.T) {
	test := func(t *testing.T, hideBanner bool) {
		// Capture stdout.
		stdout := os.Stdout
		defer func() {
			os.Stdout = stdout
		}()
		stdoutFilePath := path.Join(t.TempDir(), "stdout.txt")
		f, err := os.Create(stdoutFilePath)
		require.NoError(t, err)
		defer f.Close()
		os.Stdout = f

		cfg := pipeline.Config{
			ConduitArgs: &conduit.Args{ConduitDataDir: t.TempDir()},
			HideBanner:  hideBanner,
			Importer:    pipeline.NameConfigPair{Name: "test", Config: map[string]interface{}{"a": "a"}},
			Processors:  nil,
			Exporter:    pipeline.NameConfigPair{Name: "test", Config: map[string]interface{}{"a": "a"}},
		}
		data, err := yaml.Marshal(&cfg)
		require.NoError(t, err)
		configFile := path.Join(cfg.ConduitArgs.ConduitDataDir, conduit.DefaultConfigName)
		os.WriteFile(configFile, data, 0755)
		require.FileExists(t, configFile)

		err = runConduitCmdWithConfig(cfg.ConduitArgs)
		data, err = os.ReadFile(stdoutFilePath)
		require.NoError(t, err)

		if hideBanner {
			assert.NotContains(t, string(data), banner)
		} else {
			assert.Contains(t, string(data), banner)
		}
	}

	t.Run("Banner_hidden", func(t *testing.T) {
		test(t, true)
	})

	t.Run("Banner_shown", func(t *testing.T) {
		test(t, false)
	})
}

func TestLogFile(t *testing.T) {
	// returns stdout
	test := func(t *testing.T, logfile string) ([]byte, error) {
		// Capture stdout.
		stdout := os.Stdout
		defer func() {
			os.Stdout = stdout
		}()
		stdoutFilePath := path.Join(t.TempDir(), "stdout.txt")
		f, err := os.Create(stdoutFilePath)
		require.NoError(t, err)
		defer f.Close()
		os.Stdout = f

		cfg := pipeline.Config{
			LogFile:     logfile,
			ConduitArgs: &conduit.Args{ConduitDataDir: t.TempDir()},
			Importer:    pipeline.NameConfigPair{Name: "test", Config: map[string]interface{}{"a": "a"}},
			Processors:  nil,
			Exporter:    pipeline.NameConfigPair{Name: "test", Config: map[string]interface{}{"a": "a"}},
		}
		data, err := yaml.Marshal(&cfg)
		require.NoError(t, err)
		configFile := path.Join(cfg.ConduitArgs.ConduitDataDir, conduit.DefaultConfigName)
		os.WriteFile(configFile, data, 0755)
		require.FileExists(t, configFile)

		err = runConduitCmdWithConfig(cfg.ConduitArgs)
		return os.ReadFile(stdoutFilePath)
	}

	// logging to stdout
	t.Run("conduit-logging-stdout", func(t *testing.T) {
		data, err := test(t, "")
		require.NoError(t, err)
		dataStr := string(data)
		require.Contains(t, dataStr, "{")
		require.Contains(t, dataStr, "\nWriting logs to console.")
		require.Contains(t, dataStr, "\npipeline creation error")
	})

	// logging to file
	t.Run("conduit-logging-file", func(t *testing.T) {
		logfile := path.Join(t.TempDir(), "logfile.txt")
		data, err := test(t, logfile)
		require.NoError(t, err)
		dataStr := string(data)
		require.NotContains(t, dataStr, "{")
		logdata, err := os.ReadFile(logfile)
		require.NoError(t, err)
		logdataStr := string(logdata)
		require.Contains(t, logdataStr, "{")
		// written to stdout and logfile
		require.Contains(t, dataStr, "\npipeline creation error")
		require.Contains(t, dataStr, "\nWriting logs to file:")
		require.Contains(t, logdataStr, `"msg":"pipeline creation error`)
	})
}
