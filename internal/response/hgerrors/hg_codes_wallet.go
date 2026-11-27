package hgerrors

const (
	HGWalletBalanceInsufficient Code = 1502001
	HGWalletAccountFrozen       Code = 1502002
	HGWalletLedgerConflict      Code = 1502003
)

var hgWallet = []Definition{
	{HGWalletBalanceInsufficient, "WALLET_BALANCE_INSUFFICIENT", "M币余额不足", 409, false},
	{HGWalletAccountFrozen, "WALLET_ACCOUNT_FROZEN", "钱包已冻结", 403, false},
	{HGWalletLedgerConflict, "WALLET_LEDGER_CONFLICT", "钱包账本冲突", 409, false},
}
