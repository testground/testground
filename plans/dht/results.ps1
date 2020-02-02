param($dataDir)

if (!$dataDir) {
	$dataDir = gci $HOME/.testground/results/dht | Sort LastWriteTime | select -Last 1
}

$x = gci $dataDir -recurse | ?{$_.Name -eq "stdout.json"} | 
Get-Content | ConvertFrom-Json | %{$_.metric.value /1000000000} | ?{$_ -gt 0}
$x | measure-object -Average -Sum -Maximum -Minimum -StandardDeviation