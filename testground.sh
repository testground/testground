#!/bin/bash
# Run many testground runs, one by one, and print stats
#
# Usage:
# ./perf_test.sh <run_count> <plan> <case> <instance_count> <builder> <runner>
# e.g.
# ./perf_test.sh 50 benchmarks storm 5 docker:go local:docker

### Information gathering 
test_start_time="$(date -u +%s)"

### Reports

report_file=$2-$3-$test_start_time
mkdir -p ~/testground-reports


function assert_run_output_is_correct {

    outcome=$(awk '/run finished with outcome = / {if ($10=="failure"){print 1} else {print 0} fi}' $1)
    # $outcome is already numeric? Cannot be used as a return value for some reason
    if [ $outcome -eq 1 ] ; then
        return 1
    else
        return 0
    fi
}

function perform_runs {
    runs=$1
    plan=$2
    case=$3
    instances=$4
    builder=$5
    runner=$6

    # total number of failed test runs
    failed=0

    for i in seq 1 $1; do
        # run single, wait for it to finish and output to out.txt
        testground run single --plan $plan --testcase $case \
            --builder $5 \
            --runner $6 \
            --instances $instances --wait | tee out.txt

        # check outcome
        assert_run_output_is_correct out.txt
        outcome=$?
        failed=expr $failed + $outcome
        echo "End #$i Outcome $outcome. Failed so far: $failed"
    done

    test_end_time="$(date -u +%s)"
    total_test_time="$(($test_end_time-$test_start_time))"

    # calculate stats and exit
    success=$(($runs - $failed))
    success_rate=$(bc <<< "scale=2; ($success/$runs)*100")
    echo "===========================" >> ~/testground-reports/$report_file
    echo "Finished $1 runs" >> ~/testground-reports/$report_file
    echo "Total succeeded: $(($success))" >> ~/testground-reports/$report_file
    echo "Total failed: $failed" >> ~/testground-reports/$report_file
    echo "Success rate: $success_rate%" >> ~/testground-reports/$report_file
    echo "Total test time: $total_test_time seconds" >> ~/testground-reports/$report_file
    echo "Report file path: ~/testground-reports/$report_file "
    echo "===========================" >> ~/testground-reports/$report_file
    rm out.txt
}

# sanity check: arguments
if [ $# -lt 6 ]; then
    echo "No argument supplied: supply a number indicating number of runs, test plan, test case, instances #, builder and runner"
    exit 1
fi

perform_runs $1 $2 $3 $4 $5 $6