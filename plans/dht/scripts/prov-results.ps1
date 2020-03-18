param($dataDir)

if (!$dataDir) {
	$dataDir = gci $HOME/.testground/results/dht | Sort LastWriteTime | select -Last 1
}

$x = gci $dataDir -recurse | ?{$_.Name -eq "stdout.json"} | Get-Content | ConvertFrom-Json

$put = $x | ?{$_.metric.name -and $_.metric.name.StartsWith("time-to-provide")} | %{$_.metric.value /1000000000} | ?{$_ -gt 0}
$get = $x | ?{$_.metric.name -and $_.metric.name.StartsWith("time-to-find")} | %{$_.metric.value /1000000000} | ?{$_ -gt 0}

$put
$put | measure-object -Average -Sum -Maximum -Minimum -StandardDeviation
$get
$get | measure-object -Average -Sum -Maximum -Minimum -StandardDeviation