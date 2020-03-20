param($runID)

$ErrorActionPreference = "Stop"

#$runID = 
#$runID = "caee926a4e16"

$env:TESTGROUND_SRCDIR="$env:HOME/go/src/github.com/ipfs/testground"
$outputDir = "$env:HOME/workspace/testground/stats"
$runner = "cluster:k8s"
#$runner = "local:docker"

if (-not [System.IO.Directory]::Exists("$outputDir/$runID")) {
	$outname = "$outputDir/$runID.tar.gz"
	testground collect $runID --runner $runner -o $outname
	tar -C $outputDir -zxvf $outname
}
$groupDirs = gci $outputDir/$runID

$allFiles = $groupDirs | gci -Recurse -File
$connGraphs = $allFiles | ?{$_.Name -eq "dht_graphs.out"}
$rts = $allFiles | ?{$_.Name -eq "dht_rt.out"}
$errs = $allFiles | ?{$_.Name -eq "run.err"}

$ns = 1000000000

function basicStats ($values) {
	if ($null -eq $values) {
		return [PSCustomObject]@{
			Average = 0
			Percentile95 = 0
		}
	}
	$obj = $values | measure-object -Average -Sum -Maximum -Minimum -StandardDeviation
	$sorted = $values | Sort-Object
	$95percentile = $sorted[[math]::Ceiling(95 / 100 * ($sorted.Count - 1))]

	if ($null -eq $95percentile) {
		return "ASDFASFASF"
	} 

	return [PSCustomObject]@{
		Average = $obj.Average
		Percentile95 = $95percentile
	}
}

function groupStats ($metrics, $groupIndex) {
	$fields = @{}
	$grouped = $metrics | Group-Object -Property {$_.Name.Split("|")[$groupIndex]}
	foreach ($g in $grouped) {
		$v = basicStats($g.Group | %{$_.Value})
		$fields.Add($g.Name, $v)
	}
	$ret = New-Object -TypeName psobject -Property $fields
	return $ret
}

foreach ($groupDir in $groupDirs) {
	$groupID = $groupDir.Name

	$files =  $groupDir | gci -Recurse -File

	$queries = $files  | ?{$_.Name -eq "dht_queries.out"}
	$out = $files  | ?{$_.Name -eq "run.out"}

	$metrics = $out | Get-Content | ConvertFrom-Json | %{$_.event.metric} | ?{$_}
	$provs = $metrics |
	?{$_.name -and $_.name.StartsWith("time-to-provide") -and $_.value -gt 0} |
	%{$_.value/$ns}

	$findfirst = $metrics |
	?{$_.name -and $_.name.StartsWith("time-to-find-first")} |
	%{ [pscustomobject]@{ Name=$_.name; Value= $_.value/$ns; }}

	$findall = $metrics |
	?{$_.name -and $_.name.StartsWith("time-to-find|")} |
	%{ [pscustomobject]@{ Name=$_.name; Value= $_.value/$ns; }}

	$found = $metrics |
	?{$_.name -and $_.name.StartsWith("peers-found") -and $_.value -gt 0} |
	%{ [pscustomobject]@{ Name=$_.name; Value= $_.value; }}

	$failures = $metrics |
	?{$_.name -and $_.name.StartsWith("peers-found") -and $_.value -eq 0} |
	%{ [pscustomobject]@{ Name=$_.name; Value= $_.value; }}

	$dials = $queries | %{Get-Content $_ | ConvertFrom-Json | ?{$_.msg -eq "dialing"} | measure-object } |
	%{$_.Count} 

	$msgs = $queries | %{Get-Content $_ | ConvertFrom-Json | ?{$_.msg -eq "send"} | measure-object } |
	%{$_.Count} 

	echo "Group $groupID :"

	if ($null -ne $provs) {
		echo "Time-to-Provide"
		basicStats($provs) | Format-Table
	}

	if ($null -ne $findfirst) {
		echo "Time-to-Find-First"
		groupStats $findfirst 1 | Format-Table

		echo "Time-to-Find"
		groupStats $findall 2 | Format-Table

		echo "Peers Found"
		groupStats $found 2 | Format-Table
	
		echo "Peers Failures"
		groupStats $failures 2 | Format-Table
	}

	#echo "Total number of dials"
	basicStats($dials) | Format-Table

	#echo "Total number of messages sent"
	basicStats($msgs) | Format-Table
}

if (-not $graphs) {
	return
}

$allGraphs = $connGraphs | Get-Content | ConvertFrom-Json | Group-Object -Property msg

$allGraphs | %{
	$g = $_.Name
	$obj = $_.Group
	
    $gdata = $obj | %{"Z{0} -> Z{1};`n" -f $_.From, $_.To}
	$file = "digraph D {`n " + $gdata + "}"
	$file > "../stats/$runID/$g-conn.dot"
	
	#$file | circo "-T$fmt" -o "$g.$fmt"
}

$allRTs = $rts | Get-Content | ConvertFrom-Json | Group-Object -Property msg

$allRTs | %{
	$g = $_.Name
	$obj = $_.Group
	
    $gdata = $obj | %{"Z{0} -> Z{1};`n" -f $_.Node, $_.Peer}
	$file = "digraph D {`n " + $gdata + "}"
	$file > "../stats/$runID/$g-rt.dot"
	
	#$file | circo "-T$fmt" -o "$g.$fmt"
}