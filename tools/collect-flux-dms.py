#!/usr/bin/python3
"""
Copyright 2024 Hewlett Packard Enterprise Development LP
Other additional copyright holders may be indicated within.

The entirety of this work is licensed under the Apache License,
Version 2.0 (the "License"); you may not use this file except
in compliance with the License.

You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
"""

# The flux-coral2-dws service dumps any associated NnfDataMovement resources for a given workflow to
# its journal. This script parses the journal for these entries and runs stats on them.

import argparse
import json
import subprocess
import re
import sys

from dateutil import parser as dparser


def main():
    args = do_args()
    wf = args.workflow
    totalSize = 0
    dms = []

    p = rf"nnfdatamovement crd for workflow '{wf}': ({{.*}})$"
    r = re.compile(p)

    # Grab the flux-coral2-dws journal
    try:
        output = subprocess.check_output(
            ["journalctl", "-u", "flux-coral2-dws"]
        ).decode()
    except subprocess.CalledProcessError as e:
        print(e)
        sys.exit(e.returncode)

    # Find the line pertaining for the workflow
    for line in output.split("\n"):
        match = r.search(line)
        if match:
            dm = match.group(1)
            totalSize += len(dm.encode("utf-8"))  # count the number of bytes
            dm = json.loads(match.group(1))
            dms.append(dm)

    if len(dms) < 1:
        print(f"No NnfDataMovements found for {wf}")
        sys.exit(-1)

    starts = []
    ends = []

    for dm in dms:
        start = dm["status"]["startTime"]
        end = dm["status"]["endTime"]

        start = dparser.parse(start)
        # TODO check for Finished/Success
        end = dparser.parse(end)

        starts.append(start)
        ends.append(end)

    starts.sort()
    ends.sort()
    earliestStart = starts[0]
    latestEnd = ends[-1]
    duration = latestEnd - earliestStart

    stats = {
        "workflow": wf,
        "numDataMovements": len(dms),
        "startTime": earliestStart,
        "endTime": latestEnd,
        "duration": duration,
        "durationInSeconds": duration.total_seconds(),
        "totalSizeInBytes": totalSize,
    }

    # FIXME: Address this
    sys.stderr.write(
        """
    NOTE: This tool does not yet differentiate between DataIn/DataOut data movements. If there are
    both copy_in and copy_out directives in a workflow, then the duration spans both groups of data
    movements and would include the job runtime.
    """
    )
    print(json.dumps(stats, indent=4, default=str))


def do_args():
    parser = argparse.ArgumentParser(
        "Collect NnfDataMovement resources for a given workflow from the flux-coral2-dws journal. These entries in the journal are only created before the transition to Teardown."
    )
    parser.add_argument("workflow", type=str, help="name of the workflow")
    args = parser.parse_args()


if __name__ == "__main__":
    sys.exit(main())
