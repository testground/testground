param($graphID,$dataDir)

$allGraphs = @("br", "bt", "at")
$fmt = "png"

if (!$dataDir) {
	$dataDir = gci $HOME/.testground/results/dht | Sort LastWriteTime | select -Last 1
}

if ($graphID) {
	$allGraphs = @($graphID)
}

$allGraphs | %{
	$g = $_
	$links = gci $dataDir -recurse | ?{$_.Name -eq "stderr.json"} | Get-Content |
	ConvertFrom-Json | ?{$_.N -eq "Graph.$g"} | %{$_.M} | ConvertFrom-Json |
	%{$_.From+" -> "+$_.To+";"}
	
	$file = "digraph D { `n" + $links + "`n }"
	$file > "$g.dot"
	$file | sfdp -x -Goverlap=scale "-T$fmt" -o "$g.$fmt"
}
