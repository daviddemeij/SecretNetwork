package main

import (
	"fmt"
	"os"
	"path"

	scrt "github.com/enigmampc/SecretNetwork/types"
	sdk "github.com/enigmampc/cosmos-sdk/types"

	"github.com/enigmampc/cosmos-sdk/client"
	"github.com/enigmampc/cosmos-sdk/client/flags"
	"github.com/enigmampc/cosmos-sdk/client/keys"
	"github.com/enigmampc/cosmos-sdk/client/lcd"
	"github.com/enigmampc/cosmos-sdk/client/rpc"
	"github.com/enigmampc/cosmos-sdk/version"
	"github.com/enigmampc/cosmos-sdk/x/auth"
	authcmd "github.com/enigmampc/cosmos-sdk/x/auth/client/cli"
	authrest "github.com/enigmampc/cosmos-sdk/x/auth/client/rest"
	"github.com/enigmampc/cosmos-sdk/x/bank"
	bankcmd "github.com/enigmampc/cosmos-sdk/x/bank/client/cli"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/libs/cli"

	app "github.com/enigmampc/SecretNetwork"
)

// thanks @terra-project for this fix
const flagLegacyHdPath = "legacy-hd-path"

func main() {
	cobra.EnableCommandSorting = false

	cdc := app.MakeCodec()

	rootCmd := &cobra.Command{
		Use:   "secretcli",
		Short: "The Secret Network Client",
	}

	// Add --chain-id to persistent flags and mark it required
	rootCmd.PersistentFlags().Bool(flagLegacyHdPath, false, "Flag to specify the command uses old HD path - use this for ledger compatibility")
	rootCmd.PersistentFlags().String(flags.FlagChainID, "", "Chain ID of tendermint node")
	rootCmd.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		return initConfig(rootCmd)
	}

	// Construct Root Command
	rootCmd.AddCommand(
		rpc.StatusCommand(),
		client.ConfigCmd(app.DefaultCLIHome),
		queryCmd(cdc),
		txCmd(cdc),
		flags.LineBreak,
		lcd.ServeCommand(cdc, registerRoutes),
		flags.LineBreak,
		keys.Commands(),
		flags.LineBreak,
		version.Cmd,
		flags.NewCompletionCmd(rootCmd, true),
	)

	// Add flags and prefix all env exposed with EN
	executor := cli.PrepareMainCmd(rootCmd, "EN", app.DefaultCLIHome)

	err := executor.Execute()
	if err != nil {
		fmt.Printf("Failed executing CLI command: %s, exiting...\n", err)
		os.Exit(1)
	}
}

func queryCmd(cdc *amino.Codec) *cobra.Command {
	queryCmd := &cobra.Command{
		Use:     "query",
		Aliases: []string{"q"},
		Short:   "Querying subcommands",
	}

	queryCmd.AddCommand(
		authcmd.GetAccountCmd(cdc),
		flags.LineBreak,
		rpc.ValidatorCommand(cdc),
		rpc.BlockCommand(),
		authcmd.QueryTxsByEventsCmd(cdc),
		authcmd.QueryTxCmd(cdc), // TODO add another one like this that decrypts the output if it's from the wallet that sent the tx
		flags.LineBreak,
		S20GetQueryCmd(cdc),
	)

	// add modules' query commands
	app.ModuleBasics.AddQueryCommands(queryCmd, cdc)

	return queryCmd
}

func txCmd(cdc *amino.Codec) *cobra.Command {
	txCmd := &cobra.Command{
		Use:   "tx",
		Short: "Transactions subcommands",
	}

	viper.SetDefault(flags.FlagGasPrices, "0.25uscrt")

	txCmd.AddCommand(
		bankcmd.SendTxCmd(cdc),
		flags.LineBreak,
		authcmd.GetSignCommand(cdc),
		authcmd.GetSignDocCommand(cdc),
		authcmd.GetMultiSignCommand(cdc),
		flags.LineBreak,
		authcmd.GetBroadcastCommand(cdc),
		authcmd.GetEncodeCommand(cdc),
		authcmd.GetDecodeCommand(cdc),
		flags.LineBreak,
		S20GetTxCmd(cdc),
	)

	// add modules' tx commands
	app.ModuleBasics.AddTxCommands(txCmd, cdc)

	// remove auth and bank commands as they're mounted under the root tx command
	var cmdsToRemove []*cobra.Command

	for _, cmd := range txCmd.Commands() {
		if cmd.Use == auth.ModuleName || cmd.Use == bank.ModuleName {
			cmdsToRemove = append(cmdsToRemove, cmd)
		}
	}

	txCmd.RemoveCommand(cmdsToRemove...)

	return txCmd
}

// registerRoutes registers the routes from the different modules for the LCD.
// NOTE: details on the routes added for each module are in the module documentation
// NOTE: If making updates here you also need to update the test helper in client/lcd/test_helper.go
func registerRoutes(rs *lcd.RestServer) {
	client.RegisterRoutes(rs.CliCtx, rs.Mux)
	authrest.RegisterTxRoutes(rs.CliCtx, rs.Mux)
	app.ModuleBasics.RegisterRESTRoutes(rs.CliCtx, rs.Mux)
}

func initConfig(cmd *cobra.Command) error {
	oldHDPath, err := cmd.PersistentFlags().GetBool(flagLegacyHdPath)
	if err != nil {
		return err
	}

	// Read in the configuration file for the sdk
	config := sdk.GetConfig()

	if !oldHDPath {
		config.SetCoinType(529)
		config.SetFullFundraiserPath("44'/529'/0'/0/0")
	}

	config.SetBech32PrefixForAccount(scrt.Bech32PrefixAccAddr, scrt.Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(scrt.Bech32PrefixValAddr, scrt.Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(scrt.Bech32PrefixConsAddr, scrt.Bech32PrefixConsPub)
	config.Seal()

	home, err := cmd.PersistentFlags().GetString(cli.HomeFlag)
	if err != nil {
		return err
	}

	cfgFile := path.Join(home, "config", "config.toml")
	if _, err := os.Stat(cfgFile); err == nil {
		viper.SetConfigFile(cfgFile)

		if err := viper.ReadInConfig(); err != nil {
			return err
		}
	}
	if err := viper.BindPFlag(flags.FlagChainID, cmd.PersistentFlags().Lookup(flags.FlagChainID)); err != nil {
		return err
	}
	if err := viper.BindPFlag(cli.EncodingFlag, cmd.PersistentFlags().Lookup(cli.EncodingFlag)); err != nil {
		return err
	}
	return viper.BindPFlag(cli.OutputFlag, cmd.PersistentFlags().Lookup(cli.OutputFlag))
}
