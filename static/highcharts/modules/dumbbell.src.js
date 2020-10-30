/**
 * @license Highcharts JS v8.2.0 (2020-08-20)
 *
 * (c) 2009-2019 Sebastian Bochan, Rafal Sebestjanski
 *
 * License: www.highcharts.com/license
 */
'use strict';
(function (factory) {
    if (typeof module === 'object' && module.exports) {
        factory['default'] = factory;
        module.exports = factory;
    } else if (typeof define === 'function' && define.amd) {
        define('highcharts/modules/dumbbell', ['highcharts'], function (Highcharts) {
            factory(Highcharts);
            factory.Highcharts = Highcharts;
            return factory;
        });
    } else {
        factory(typeof Highcharts !== 'undefined' ? Highcharts : undefined);
    }
}(function (Highcharts) {
    var _modules = Highcharts ? Highcharts._modules : {};
    function _registerModule(obj, path, args, fn) {
        if (!obj.hasOwnProperty(path)) {
            obj[path] = fn.apply(null, args);
        }
    }
    _registerModule(_modules, 'Series/AreaRangeSeries.js', [_modules['Core/Globals.js'], _modules['Core/Series/Point.js'], _modules['Core/Utilities.js']], function (H, Point, U) {
        /* *
         *
         *  (c) 2010-2020 Torstein Honsi
         *
         *  License: www.highcharts.com/license
         *
         *  !!!!!!! SOURCE GETS TRANSPILED BY TYPESCRIPT. EDIT TS FILE ONLY. !!!!!!!
         *
         * */
        var defined = U.defined,
            extend = U.extend,
            isArray = U.isArray,
            isNumber = U.isNumber,
            pick = U.pick,
            seriesType = U.seriesType;
        var noop = H.noop,
            Series = H.Series,
            seriesTypes = H.seriesTypes,
            seriesProto = Series.prototype,
            pointProto = Point.prototype;
        /**
         * The area range series is a carteseian series with higher and lower values for
         * each point along an X axis, where the area between the values is shaded.
         *
         * @sample {highcharts} highcharts/demo/arearange/
         *         Area range chart
         * @sample {highstock} stock/demo/arearange/
         *         Area range chart
         *
         * @extends      plotOptions.area
         * @product      highcharts highstock
         * @excluding    stack, stacking
         * @requires     highcharts-more
         * @optionparent plotOptions.arearange
         */
        seriesType('arearange', 'area', {
            /**
             * Whether to apply a drop shadow to the graph line. Since 2.3 the shadow
             * can be an object configuration containing `color`, `offsetX`, `offsetY`,
             * `opacity` and `width`.
             *
             * @type      {boolean|Highcharts.ShadowOptionsObject}
             * @product   highcharts
             * @apioption plotOptions.arearange.shadow
             */
            /**
             * @default   low
             * @apioption plotOptions.arearange.colorKey
             */
            /**
             * Pixel width of the arearange graph line.
             *
             * @since 2.3.0
             *
             * @private
             */
            lineWidth: 1,
            threshold: null,
            tooltip: {
                pointFormat: '<span style="color:{series.color}">\u25CF</span> ' +
                    '{series.name}: <b>{point.low}</b> - <b>{point.high}</b><br/>'
            },
            /**
             * Whether the whole area or just the line should respond to mouseover
             * tooltips and other mouse or touch events.
             *
             * @since 2.3.0
             *
             * @private
             */
            trackByArea: true,
            /**
             * Extended data labels for range series types. Range series data labels use
             * no `x` and `y` options. Instead, they have `xLow`, `xHigh`, `yLow` and
             * `yHigh` options to allow the higher and lower data label sets
             * individually.
             *
             * @declare Highcharts.SeriesAreaRangeDataLabelsOptionsObject
             * @exclude x, y
             * @since   2.3.0
             * @product highcharts highstock
             *
             * @private
             */
            dataLabels: {
                align: void 0,
                verticalAlign: void 0,
                /**
                 * X offset of the lower data labels relative to the point value.
                 *
                 * @sample highcharts/plotoptions/arearange-datalabels/
                 *         Data labels on range series
                 * @sample highcharts/plotoptions/arearange-datalabels/
                 *         Data labels on range series
                 */
                xLow: 0,
                /**
                 * X offset of the higher data labels relative to the point value.
                 *
                 * @sample highcharts/plotoptions/arearange-datalabels/
                 *         Data labels on range series
                 */
                xHigh: 0,
                /**
                 * Y offset of the lower data labels relative to the point value.
                 *
                 * @sample highcharts/plotoptions/arearange-datalabels/
                 *         Data labels on range series
                 */
                yLow: 0,
                /**
                 * Y offset of the higher data labels relative to the point value.
                 *
                 * @sample highcharts/plotoptions/arearange-datalabels/
                 *         Data labels on range series
                 */
                yHigh: 0
            }
            // Prototype members
        }, {
            pointArrayMap: ['low', 'high'],
            pointValKey: 'low',
            deferTranslatePolar: true,
            /* eslint-disable valid-jsdoc */
            /**
             * @private
             */
            toYData: function (point) {
                return [point.low, point.high];
            },
            /**
             * Translate a point's plotHigh from the internal angle and radius measures
             * to true plotHigh coordinates. This is an addition of the toXY method
             * found in Polar.js, because it runs too early for arearanges to be
             * considered (#3419).
             * @private
             */
            highToXY: function (point) {
                // Find the polar plotX and plotY
                var chart = this.chart,
                    xy = this.xAxis.postTranslate(point.rectPlotX,
                    this.yAxis.len - point.plotHigh);
                point.plotHighX = xy.x - chart.plotLeft;
                point.plotHigh = xy.y - chart.plotTop;
                point.plotLowX = point.plotX;
            },
            /**
             * Translate data points from raw values x and y to plotX and plotY.
             * @private
             */
            translate: function () {
                var series = this,
                    yAxis = series.yAxis,
                    hasModifyValue = !!series.modifyValue;
                seriesTypes.area.prototype.translate.apply(series);
                // Set plotLow and plotHigh
                series.points.forEach(function (point) {
                    var high = point.high,
                        plotY = point.plotY;
                    if (point.isNull) {
                        point.plotY = null;
                    }
                    else {
                        point.plotLow = plotY;
                        point.plotHigh = yAxis.translate(hasModifyValue ?
                            series.modifyValue(high, point) :
                            high, 0, 1, 0, 1);
                        if (hasModifyValue) {
                            point.yBottom = point.plotHigh;
                        }
                    }
                });
                // Postprocess plotHigh
                if (this.chart.polar) {
                    this.points.forEach(function (point) {
                        series.highToXY(point);
                        point.tooltipPos = [
                            (point.plotHighX + point.plotLowX) / 2,
                            (point.plotHigh + point.plotLow) / 2
                        ];
                    });
                }
            },
            /**
             * Extend the line series' getSegmentPath method by applying the segment
             * path to both lower and higher values of the range.
             * @private
             */
            getGraphPath: function (points) {
                var highPoints = [],
                    highAreaPoints = [],
                    i,
                    getGraphPath = seriesTypes.area.prototype.getGraphPath,
                    point,
                    pointShim,
                    linePath,
                    lowerPath,
                    options = this.options,
                    polar = this.chart.polar,
                    connectEnds = polar && options.connectEnds !== false,
                    connectNulls = options.connectNulls,
                    step = options.step,
                    higherPath,
                    higherAreaPath;
                points = points || this.points;
                // Create the top line and the top part of the area fill. The area fill
                // compensates for null points by drawing down to the lower graph,
                // moving across the null gap and starting again at the lower graph.
                i = points.length;
                while (i--) {
                    point = points[i];
                    // Support for polar
                    var highAreaPoint = polar ? {
                            plotX: point.rectPlotX,
                            plotY: point.yBottom,
                            doCurve: false // #5186, gaps in areasplinerange fill
                        } : {
                            plotX: point.plotX,
                            plotY: point.plotY,
                            doCurve: false // #5186, gaps in areasplinerange fill
                        };
                    if (!point.isNull &&
                        !connectEnds &&
                        !connectNulls &&
                        (!points[i + 1] || points[i + 1].isNull)) {
                        highAreaPoints.push(highAreaPoint);
                    }
                    pointShim = {
                        polarPlotY: point.polarPlotY,
                        rectPlotX: point.rectPlotX,
                        yBottom: point.yBottom,
                        // plotHighX is for polar charts
                        plotX: pick(point.plotHighX, point.plotX),
                        plotY: point.plotHigh,
                        isNull: point.isNull
                    };
                    highAreaPoints.push(pointShim);
                    highPoints.push(pointShim);
                    if (!point.isNull &&
                        !connectEnds &&
                        !connectNulls &&
                        (!points[i - 1] || points[i - 1].isNull)) {
                        highAreaPoints.push(highAreaPoint);
                    }
                }
                // Get the paths
                lowerPath = getGraphPath.call(this, points);
                if (step) {
                    if (step === true) {
                        step = 'left';
                    }
                    options.step = {
                        left: 'right',
                        center: 'center',
                        right: 'left'
                    }[step]; // swap for reading in getGraphPath
                }
                higherPath = getGraphPath.call(this, highPoints);
                higherAreaPath = getGraphPath.call(this, highAreaPoints);
                options.step = step;
                // Create a line on both top and bottom of the range
                linePath = []
                    .concat(lowerPath, higherPath);
                // For the area path, we need to change the 'move' statement
                // into 'lineTo'
                if (!this.chart.polar && higherAreaPath[0] && higherAreaPath[0][0] === 'M') {
                    // This probably doesn't work for spline
                    higherAreaPath[0] = ['L', higherAreaPath[0][1], higherAreaPath[0][2]];
                }
                this.graphPath = linePath;
                this.areaPath = lowerPath.concat(higherAreaPath);
                // Prepare for sideways animation
                linePath.isArea = true;
                linePath.xMap = lowerPath.xMap;
                this.areaPath.xMap = lowerPath.xMap;
                return linePath;
            },
            /**
             * Extend the basic drawDataLabels method by running it for both lower and
             * higher values.
             * @private
             */
            drawDataLabels: function () {
                var data = this.points,
                    length = data.length,
                    i,
                    originalDataLabels = [],
                    dataLabelOptions = this.options.dataLabels,
                    point,
                    up,
                    inverted = this.chart.inverted,
                    upperDataLabelOptions,
                    lowerDataLabelOptions;
                // Split into upper and lower options. If data labels is an array, the
                // first element is the upper label, the second is the lower.
                //
                // TODO: We want to change this and allow multiple labels for both upper
                // and lower values in the future - introducing some options for which
                // point value to use as Y for the dataLabel, so that this could be
                // handled in Series.drawDataLabels. This would also improve performance
                // since we now have to loop over all the points multiple times to work
                // around the data label logic.
                if (isArray(dataLabelOptions)) {
                    if (dataLabelOptions.length > 1) {
                        upperDataLabelOptions = dataLabelOptions[0];
                        lowerDataLabelOptions = dataLabelOptions[1];
                    }
                    else {
                        upperDataLabelOptions = dataLabelOptions[0];
                        lowerDataLabelOptions = { enabled: false };
                    }
                }
                else {
                    // Make copies
                    upperDataLabelOptions = extend({}, dataLabelOptions);
                    upperDataLabelOptions.x = dataLabelOptions.xHigh;
                    upperDataLabelOptions.y = dataLabelOptions.yHigh;
                    lowerDataLabelOptions = extend({}, dataLabelOptions);
                    lowerDataLabelOptions.x = dataLabelOptions.xLow;
                    lowerDataLabelOptions.y = dataLabelOptions.yLow;
                }
                // Draw upper labels
                if (upperDataLabelOptions.enabled || this._hasPointLabels) {
                    // Set preliminary values for plotY and dataLabel
                    // and draw the upper labels
                    i = length;
                    while (i--) {
                        point = data[i];
                        if (point) {
                            up = upperDataLabelOptions.inside ?
                                point.plotHigh < point.plotLow :
                                point.plotHigh > point.plotLow;
                            point.y = point.high;
                            point._plotY = point.plotY;
                            point.plotY = point.plotHigh;
                            // Store original data labels and set preliminary label
                            // objects to be picked up in the uber method
                            originalDataLabels[i] = point.dataLabel;
                            point.dataLabel = point.dataLabelUpper;
                            // Set the default offset
                            point.below = up;
                            if (inverted) {
                                if (!upperDataLabelOptions.align) {
                                    upperDataLabelOptions.align = up ? 'right' : 'left';
                                }
                            }
                            else {
                                if (!upperDataLabelOptions.verticalAlign) {
                                    upperDataLabelOptions.verticalAlign = up ?
                                        'top' :
                                        'bottom';
                                }
                            }
                        }
                    }
                    this.options.dataLabels = upperDataLabelOptions;
                    if (seriesProto.drawDataLabels) {
                        // #1209:
                        seriesProto.drawDataLabels.apply(this, arguments);
                    }
                    // Reset state after the upper labels were created. Move
                    // it to point.dataLabelUpper and reassign the originals.
                    // We do this here to support not drawing a lower label.
                    i = length;
                    while (i--) {
                        point = data[i];
                        if (point) {
                            point.dataLabelUpper = point.dataLabel;
                            point.dataLabel = originalDataLabels[i];
                            delete point.dataLabels;
                            point.y = point.low;
                            point.plotY = point._plotY;
                        }
                    }
                }
                // Draw lower labels
                if (lowerDataLabelOptions.enabled || this._hasPointLabels) {
                    i = length;
                    while (i--) {
                        point = data[i];
                        if (point) {
                            up = lowerDataLabelOptions.inside ?
                                point.plotHigh < point.plotLow :
                                point.plotHigh > point.plotLow;
                            // Set the default offset
                            point.below = !up;
                            if (inverted) {
                                if (!lowerDataLabelOptions.align) {
                                    lowerDataLabelOptions.align = up ? 'left' : 'right';
                                }
                            }
                            else {
                                if (!lowerDataLabelOptions.verticalAlign) {
                                    lowerDataLabelOptions.verticalAlign = up ?
                                        'bottom' :
                                        'top';
                                }
                            }
                        }
                    }
                    this.options.dataLabels = lowerDataLabelOptions;
                    if (seriesProto.drawDataLabels) {
                        seriesProto.drawDataLabels.apply(this, arguments);
                    }
                }
                // Merge upper and lower into point.dataLabels for later destroying
                if (upperDataLabelOptions.enabled) {
                    i = length;
                    while (i--) {
                        point = data[i];
                        if (point) {
                            point.dataLabels = [
                                point.dataLabelUpper,
                                point.dataLabel
                            ].filter(function (label) {
                                return !!label;
                            });
                        }
                    }
                }
                // Reset options
                this.options.dataLabels = dataLabelOptions;
            },
            alignDataLabel: function () {
                seriesTypes.column.prototype.alignDataLabel
                    .apply(this, arguments);
            },
            drawPoints: function () {
                var series = this,
                    pointLength = series.points.length,
                    point,
                    i;
                // Draw bottom points
                seriesProto.drawPoints
                    .apply(series, arguments);
                // Prepare drawing top points
                i = 0;
                while (i < pointLength) {
                    point = series.points[i];
                    // Save original props to be overridden by temporary props for top
                    // points
                    point.origProps = {
                        plotY: point.plotY,
                        plotX: point.plotX,
                        isInside: point.isInside,
                        negative: point.negative,
                        zone: point.zone,
                        y: point.y
                    };
                    point.lowerGraphic = point.graphic;
                    point.graphic = point.upperGraphic;
                    point.plotY = point.plotHigh;
                    if (defined(point.plotHighX)) {
                        point.plotX = point.plotHighX;
                    }
                    point.y = point.high;
                    point.negative = point.high < (series.options.threshold || 0);
                    point.zone = (series.zones.length && point.getZone());
                    if (!series.chart.polar) {
                        point.isInside = point.isTopInside = (typeof point.plotY !== 'undefined' &&
                            point.plotY >= 0 &&
                            point.plotY <= series.yAxis.len && // #3519
                            point.plotX >= 0 &&
                            point.plotX <= series.xAxis.len);
                    }
                    i++;
                }
                // Draw top points
                seriesProto.drawPoints.apply(series, arguments);
                // Reset top points preliminary modifications
                i = 0;
                while (i < pointLength) {
                    point = series.points[i];
                    point.upperGraphic = point.graphic;
                    point.graphic = point.lowerGraphic;
                    extend(point, point.origProps);
                    delete point.origProps;
                    i++;
                }
            },
            /* eslint-enable valid-jsdoc */
            setStackedPoints: noop
        }, {
            /**
             * Range series only. The high or maximum value for each data point.
             * @name Highcharts.Point#high
             * @type {number|undefined}
             */
            /**
             * Range series only. The low or minimum value for each data point.
             * @name Highcharts.Point#low
             * @type {number|undefined}
             */
            /* eslint-disable valid-jsdoc */
            /**
             * @private
             */
            setState: function () {
                var prevState = this.state,
                    series = this.series,
                    isPolar = series.chart.polar;
                if (!defined(this.plotHigh)) {
                    // Boost doesn't calculate plotHigh
                    this.plotHigh = series.yAxis.toPixels(this.high, true);
                }
                if (!defined(this.plotLow)) {
                    // Boost doesn't calculate plotLow
                    this.plotLow = this.plotY = series.yAxis.toPixels(this.low, true);
                }
                if (series.stateMarkerGraphic) {
                    series.lowerStateMarkerGraphic = series.stateMarkerGraphic;
                    series.stateMarkerGraphic = series.upperStateMarkerGraphic;
                }
                // Change state also for the top marker
                this.graphic = this.upperGraphic;
                this.plotY = this.plotHigh;
                if (isPolar) {
                    this.plotX = this.plotHighX;
                }
                // Top state:
                pointProto.setState.apply(this, arguments);
                this.state = prevState;
                // Now restore defaults
                this.plotY = this.plotLow;
                this.graphic = this.lowerGraphic;
                if (isPolar) {
                    this.plotX = this.plotLowX;
                }
                if (series.stateMarkerGraphic) {
                    series.upperStateMarkerGraphic = series.stateMarkerGraphic;
                    series.stateMarkerGraphic = series.lowerStateMarkerGraphic;
                    // Lower marker is stored at stateMarkerGraphic
                    // to avoid reference duplication (#7021)
                    series.lowerStateMarkerGraphic = void 0;
                }
                pointProto.setState.apply(this, arguments);
            },
            haloPath: function () {
                var isPolar = this.series.chart.polar,
                    path = [];
                // Bottom halo
                this.plotY = this.plotLow;
                if (isPolar) {
                    this.plotX = this.plotLowX;
                }
                if (this.isInside) {
                    path = pointProto.haloPath.apply(this, arguments);
                }
                // Top halo
                this.plotY = this.plotHigh;
                if (isPolar) {
                    this.plotX = this.plotHighX;
                }
                if (this.isTopInside) {
                    path = path.concat(pointProto.haloPath.apply(this, arguments));
                }
                return path;
            },
            destroyElements: function () {
                var graphics = ['lowerGraphic', 'upperGraphic'];
                graphics.forEach(function (graphicName) {
                    if (this[graphicName]) {
                        this[graphicName] =
                            this[graphicName].destroy();
                    }
                }, this);
                // Clear graphic for states, removed in the above each:
                this.graphic = null;
                return pointProto.destroyElements.apply(this, arguments);
            },
            isValid: function () {
                return isNumber(this.low) && isNumber(this.high);
            }
            /* eslint-enable valid-jsdoc */
        });
        /**
         * A `arearange` series. If the [type](#series.arearange.type) option is not
         * specified, it is inherited from [chart.type](#chart.type).
         *
         *
         * @extends   series,plotOptions.arearange
         * @excluding dataParser, dataURL, stack, stacking
         * @product   highcharts highstock
         * @requires  highcharts-more
         * @apioption series.arearange
         */
        /**
         * An array of data points for the series. For the `arearange` series type,
         * points can be given in the following ways:
         *
         * 1.  An array of arrays with 3 or 2 values. In this case, the values
         *     correspond to `x,low,high`. If the first value is a string, it is
         *     applied as the name of the point, and the `x` value is inferred.
         *     The `x` value can also be omitted, in which case the inner arrays
         *     should be of length 2\. Then the `x` value is automatically calculated,
         *     either starting at 0 and incremented by 1, or from `pointStart`
         *     and `pointInterval` given in the series options.
         *     ```js
         *     data: [
         *         [0, 8, 3],
         *         [1, 1, 1],
         *         [2, 6, 8]
         *     ]
         *     ```
         *
         * 2.  An array of objects with named values. The following snippet shows only a
         *     few settings, see the complete options set below. If the total number of
         *     data points exceeds the series'
         *     [turboThreshold](#series.arearange.turboThreshold),
         *     this option is not available.
         *     ```js
         *     data: [{
         *         x: 1,
         *         low: 9,
         *         high: 0,
         *         name: "Point2",
         *         color: "#00FF00"
         *     }, {
         *         x: 1,
         *         low: 3,
         *         high: 4,
         *         name: "Point1",
         *         color: "#FF00FF"
         *     }]
         *     ```
         *
         * @sample {highcharts} highcharts/series/data-array-of-arrays/
         *         Arrays of numeric x and y
         * @sample {highcharts} highcharts/series/data-array-of-arrays-datetime/
         *         Arrays of datetime x and y
         * @sample {highcharts} highcharts/series/data-array-of-name-value/
         *         Arrays of point.name and y
         * @sample {highcharts} highcharts/series/data-array-of-objects/
         *         Config objects
         *
         * @type      {Array<Array<(number|string),number>|Array<(number|string),number,number>|*>}
         * @extends   series.line.data
         * @excluding marker, y
         * @product   highcharts highstock
         * @apioption series.arearange.data
         */
        /**
         * @extends   series.arearange.dataLabels
         * @product   highcharts highstock
         * @apioption series.arearange.data.dataLabels
         */
        /**
         * The high or maximum value for each data point.
         *
         * @type      {number}
         * @product   highcharts highstock
         * @apioption series.arearange.data.high
         */
        /**
         * The low or minimum value for each data point.
         *
         * @type      {number}
         * @product   highcharts highstock
         * @apioption series.arearange.data.low
         */
        ''; // adds doclets above to tranpiled file

    });
    _registerModule(_modules, 'Series/DumbbellSeries.js', [_modules['Core/Globals.js'], _modules['Core/Utilities.js']], function (H, U) {
        /* *
         *
         *  (c) 2010-2020 Sebastian Bochan, Rafal Sebestjanski
         *
         *  License: www.highcharts.com/license
         *
         *  !!!!!!! SOURCE GETS TRANSPILED BY TYPESCRIPT. EDIT TS FILE ONLY. !!!!!!!
         *
         * */
        var SVGRenderer = H.SVGRenderer;
        var extend = U.extend,
            pick = U.pick,
            seriesType = U.seriesType;
        var seriesTypes = H.seriesTypes,
            seriesProto = H.Series.prototype,
            areaRangeProto = seriesTypes.arearange.prototype,
            columnRangeProto = seriesTypes.columnrange.prototype,
            colProto = seriesTypes.column.prototype,
            areaRangePointProto = areaRangeProto.pointClass.prototype;
        /**
         * The dumbbell series is a cartesian series with higher and lower values for
         * each point along an X axis, connected with a line between the values.
         * Requires `highcharts-more.js` and `modules/dumbbell.js`.
         *
         * @sample {highcharts} highcharts/demo/dumbbell/
         *         Dumbbell chart
         * @sample {highcharts} highcharts/series-dumbbell/styled-mode-dumbbell/
         *         Styled mode
         *
         * @extends      plotOptions.arearange
         * @product      highcharts highstock
         * @excluding    fillColor, fillOpacity, lineWidth, stack, stacking,
         *               stickyTracking, trackByArea, boostThreshold, boostBlending
         * @since 8.0.0
         * @optionparent plotOptions.dumbbell
         */
        seriesType('dumbbell', 'arearange', {
            /** @ignore-option */
            trackByArea: false,
            /** @ignore-option */
            fillColor: 'none',
            /** @ignore-option */
            lineWidth: 0,
            pointRange: 1,
            /**
             * Pixel width of the line that connects the dumbbell point's values.
             *
             * @since 8.0.0
             * @product   highcharts highstock
             */
            connectorWidth: 1,
            /** @ignore-option */
            stickyTracking: false,
            groupPadding: 0.2,
            crisp: false,
            pointPadding: 0.1,
            /**
             * Color of the start markers in a dumbbell graph.
             *
             * @type      {Highcharts.ColorString|Highcharts.GradientColorObject|Highcharts.PatternObject}
             * @since 8.0.0
             * @product   highcharts highstock
             */
            lowColor: '#333333',
            /**
             * Color of the line that connects the dumbbell point's values.
             * By default it is the series' color.
             *
             * @type      {string}
             * @product   highcharts highstock
             * @since 8.0.0
             * @apioption plotOptions.dumbbell.connectorColor
             */
            states: {
                hover: {
                    /** @ignore-option */
                    lineWidthPlus: 0,
                    /**
                     * The additional connector line width for a hovered point.
                     *
                     * @since 8.0.0
                     * @product   highcharts highstock
                     */
                    connectorWidthPlus: 1,
                    /** @ignore-option */
                    halo: false
                }
            }
        }, {
            trackerGroups: ['group', 'markerGroup', 'dataLabelsGroup'],
            drawTracker: H.TrackerMixin.drawTrackerPoint,
            drawGraph: H.noop,
            crispCol: colProto.crispCol,
            /**
             * Get connector line path and styles that connects dumbbell point's low and
             * high values.
             * @private
             *
             * @param {Highcharts.Series} this The series of points.
             * @param {Highcharts.Point} point The point to inspect.
             *
             * @return {Highcharts.SVGAttributes} attribs The path and styles.
             */
            getConnectorAttribs: function (point) {
                var series = this,
                    chart = series.chart,
                    pointOptions = point.options,
                    seriesOptions = series.options,
                    xAxis = series.xAxis,
                    yAxis = series.yAxis,
                    connectorWidth = pick(pointOptions.connectorWidth,
                    seriesOptions.connectorWidth),
                    connectorColor = pick(pointOptions.connectorColor,
                    seriesOptions.connectorColor,
                    pointOptions.color,
                    point.zone ? point.zone.color : void 0,
                    point.color),
                    connectorWidthPlus = pick(seriesOptions.states &&
                        seriesOptions.states.hover &&
                        seriesOptions.states.hover.connectorWidthPlus, 1),
                    dashStyle = pick(pointOptions.dashStyle,
                    seriesOptions.dashStyle),
                    pointTop = pick(point.plotLow,
                    point.plotY),
                    pxThreshold = yAxis.toPixels(seriesOptions.threshold || 0,
                    true),
                    pointHeight = chart.inverted ?
                        yAxis.len - pxThreshold : pxThreshold,
                    pointBottom = pick(point.plotHigh,
                    pointHeight),
                    attribs,
                    origProps;
                if (point.state) {
                    connectorWidth = connectorWidth + connectorWidthPlus;
                }
                if (pointTop < 0) {
                    pointTop = 0;
                }
                else if (pointTop >= yAxis.len) {
                    pointTop = yAxis.len;
                }
                if (pointBottom < 0) {
                    pointBottom = 0;
                }
                else if (pointBottom >= yAxis.len) {
                    pointBottom = yAxis.len;
                }
                if (point.plotX < 0 || point.plotX > xAxis.len) {
                    connectorWidth = 0;
                }
                // Connector should reflect upper marker's zone color
                if (point.upperGraphic) {
                    origProps = {
                        y: point.y,
                        zone: point.zone
                    };
                    point.y = point.high;
                    point.zone = point.zone ? point.getZone() : void 0;
                    connectorColor = pick(pointOptions.connectorColor, seriesOptions.connectorColor, pointOptions.color, point.zone ? point.zone.color : void 0, point.color);
                    extend(point, origProps);
                }
                attribs = {
                    d: SVGRenderer.prototype.crispLine([[
                            'M',
                            point.plotX,
                            pointTop
                        ], [
                            'L',
                            point.plotX,
                            pointBottom
                        ]], connectorWidth, 'ceil')
                };
                if (!chart.styledMode) {
                    attribs.stroke = connectorColor;
                    attribs['stroke-width'] = connectorWidth;
                    if (dashStyle) {
                        attribs.dashstyle = dashStyle;
                    }
                }
                return attribs;
            },
            /**
             * Draw connector line that connects dumbbell point's low and high values.
             * @private
             *
             * @param {Highcharts.Series} this The series of points.
             * @param {Highcharts.Point} point The point to inspect.
             *
             * @return {void}
             */
            drawConnector: function (point) {
                var series = this,
                    animationLimit = pick(series.options.animationLimit, 250),
                    verb = point.connector && series.chart.pointCount < animationLimit ?
                        'animate' : 'attr';
                if (!point.connector) {
                    point.connector = series.chart.renderer.path()
                        .addClass('highcharts-lollipop-stem')
                        .attr({
                        zIndex: -1
                    })
                        .add(series.markerGroup);
                }
                point.connector[verb](this.getConnectorAttribs(point));
            },
            /**
             * Return the width and x offset of the dumbbell adjusted for grouping,
             * groupPadding, pointPadding, pointWidth etc.
             *
             * @private
             *
             * @function Highcharts.seriesTypes.column#getColumnMetrics
             *
             * @param {Highcharts.Series} this The series of points.
             *
             * @return {Highcharts.ColumnMetricsObject} metrics shapeArgs
             *
             */
            getColumnMetrics: function () {
                var metrics = colProto.getColumnMetrics.apply(this,
                    arguments);
                metrics.offset += metrics.width / 2;
                return metrics;
            },
            translatePoint: areaRangeProto.translate,
            setShapeArgs: columnRangeProto.translate,
            /**
             * Translate each point to the plot area coordinate system and find
             * shape positions
             *
             * @private
             *
             * @function Highcharts.seriesTypes.dumbbell#translate
             *
             * @param {Highcharts.Series} this The series of points.
             *
             * @return {void}
             */
            translate: function () {
                // Calculate shapeargs
                this.setShapeArgs.apply(this);
                // Calculate point low / high values
                this.translatePoint.apply(this, arguments);
                // Correct x position
                this.points.forEach(function (point) {
                    var shapeArgs = point.shapeArgs,
                        pointWidth = point.pointWidth;
                    point.plotX = shapeArgs.x;
                    shapeArgs.x = point.plotX - pointWidth / 2;
                    point.tooltipPos = null;
                });
                this.columnMetrics.offset -= this.columnMetrics.width / 2;
            },
            seriesDrawPoints: areaRangeProto.drawPoints,
            /**
             * Extend the arearange series' drawPoints method by applying a connector
             * and coloring markers.
             * @private
             *
             * @function Highcharts.Series#drawPoints
             *
             * @param {Highcharts.Series} this The series of points.
             *
             * @return {void}
             */
            drawPoints: function () {
                var series = this,
                    chart = series.chart,
                    pointLength = series.points.length,
                    seriesLowColor = series.lowColor = series.options.lowColor,
                    i = 0,
                    lowerGraphicColor,
                    point,
                    zoneColor;
                this.seriesDrawPoints.apply(series, arguments);
                // Draw connectors and color upper markers
                while (i < pointLength) {
                    point = series.points[i];
                    series.drawConnector(point);
                    if (point.upperGraphic) {
                        point.upperGraphic.element.point = point;
                        point.upperGraphic.addClass('highcharts-lollipop-high');
                    }
                    point.connector.element.point = point;
                    if (point.lowerGraphic) {
                        zoneColor = point.zone && point.zone.color;
                        lowerGraphicColor = pick(point.options.lowColor, seriesLowColor, point.options.color, zoneColor, point.color, series.color);
                        if (!chart.styledMode) {
                            point.lowerGraphic.attr({
                                fill: lowerGraphicColor
                            });
                        }
                        point.lowerGraphic.addClass('highcharts-lollipop-low');
                    }
                    i++;
                }
            },
            /**
             * Get non-presentational attributes for a point. Used internally for
             * both styled mode and classic. Set correct position in link with connector
             * line.
             *
             * @see Series#pointAttribs
             *
             * @function Highcharts.Series#markerAttribs
             *
             * @param {Highcharts.Series} this The series of points.
             *
             * @return {Highcharts.SVGAttributes}
             *         A hash containing those attributes that are not settable from
             *         CSS.
             */
            markerAttribs: function () {
                var ret = areaRangeProto.markerAttribs.apply(this,
                    arguments);
                ret.x = Math.floor(ret.x);
                ret.y = Math.floor(ret.y);
                return ret;
            },
            /**
             * Get presentational attributes
             *
             * @private
             * @function Highcharts.seriesTypes.column#pointAttribs
             *
             * @param {Highcharts.Series} this The series of points.
             * @param {Highcharts.Point} point The point to inspect.
             * @param {string} state current state of point (normal, hover, select)
             *
             * @return {Highcharts.SVGAttributes} pointAttribs SVGAttributes
             */
            pointAttribs: function (point, state) {
                var pointAttribs;
                pointAttribs = seriesProto.pointAttribs.apply(this, arguments);
                if (state === 'hover') {
                    delete pointAttribs.fill;
                }
                return pointAttribs;
            }
        }, {
            // seriesTypes doesn't inherit from arearange point proto so put below
            // methods rigidly.
            destroyElements: areaRangePointProto.destroyElements,
            isValid: areaRangePointProto.isValid,
            pointSetState: areaRangePointProto.setState,
            /**
             * Set the point's state extended by have influence on the connector
             * (between low and high value).
             *
             * @private
             * @param {Highcharts.Point} this The point to inspect.
             *
             * @return {void}
             */
            setState: function () {
                var point = this,
                    series = point.series,
                    chart = series.chart,
                    seriesLowColor = series.options.lowColor,
                    seriesMarker = series.options.marker,
                    pointOptions = point.options,
                    pointLowColor = pointOptions.lowColor,
                    zoneColor = point.zone && point.zone.color,
                    lowerGraphicColor = pick(pointLowColor,
                    seriesLowColor,
                    pointOptions.color,
                    zoneColor,
                    point.color,
                    series.color),
                    verb = 'attr',
                    upperGraphicColor,
                    origProps;
                this.pointSetState.apply(this, arguments);
                if (!point.state) {
                    verb = 'animate';
                    if (point.lowerGraphic && !chart.styledMode) {
                        point.lowerGraphic.attr({
                            fill: lowerGraphicColor
                        });
                        if (point.upperGraphic) {
                            origProps = {
                                y: point.y,
                                zone: point.zone
                            };
                            point.y = point.high;
                            point.zone = point.zone ? point.getZone() : void 0;
                            upperGraphicColor = pick(point.marker ? point.marker.fillColor : void 0, seriesMarker ? seriesMarker.fillColor : void 0, pointOptions.color, point.zone ? point.zone.color : void 0, point.color);
                            point.upperGraphic.attr({
                                fill: upperGraphicColor
                            });
                            extend(point, origProps);
                        }
                    }
                }
                point.connector[verb](series.getConnectorAttribs(point));
            }
        });
        /**
         * The `dumbbell` series. If the [type](#series.dumbbell.type) option is
         * not specified, it is inherited from [chart.type](#chart.type).
         *
         * @extends   series,plotOptions.dumbbell
         * @excluding boostThreshold, boostBlending
         * @product   highcharts highstock
         * @requires  highcharts-more
         * @requires  modules/dumbbell
         * @apioption series.dumbbell
         */
        /**
         * An array of data points for the series. For the `dumbbell` series
         * type, points can be given in the following ways:
         *
         * 1. An array of arrays with 3 or 2 values. In this case, the values correspond
         *    to `x,low,high`. If the first value is a string, it is applied as the name
         *    of the point, and the `x` value is inferred. The `x` value can also be
         *    omitted, in which case the inner arrays should be of length 2\. Then the
         *    `x` value is automatically calculated, either starting at 0 and
         *    incremented by 1, or from `pointStart` and `pointInterval` given in the
         *    series options.
         *    ```js
         *    data: [
         *        [0, 4, 2],
         *        [1, 2, 1],
         *        [2, 9, 10]
         *    ]
         *    ```
         *
         * 2. An array of objects with named values. The following snippet shows only a
         *    few settings, see the complete options set below. If the total number of
         *    data points exceeds the series'
         *    [turboThreshold](#series.dumbbell.turboThreshold), this option is not
         *    available.
         *    ```js
         *    data: [{
         *        x: 1,
         *        low: 0,
         *        high: 4,
         *        name: "Point2",
         *        color: "#00FF00",
         *        lowColor: "#00FFFF",
         *        connectorWidth: 3,
         *        connectorColor: "#FF00FF"
         *    }, {
         *        x: 1,
         *        low: 5,
         *        high: 3,
         *        name: "Point1",
         *        color: "#FF00FF"
         *    }]
         *    ```
         *
         * @sample {highcharts} highcharts/series/data-array-of-arrays/
         *         Arrays of numeric x and y
         * @sample {highcharts} highcharts/series/data-array-of-arrays-datetime/
         *         Arrays of datetime x and y
         * @sample {highcharts} highcharts/series/data-array-of-name-value/
         *         Arrays of point.name and y
         * @sample {highcharts} highcharts/series/data-array-of-objects/
         *         Config objects
         *
         * @type      {Array<Array<(number|string),number>|Array<(number|string),number,number>|*>}
         * @extends   series.arearange.data
         * @product   highcharts highstock
         * @apioption series.dumbbell.data
         */
        /**
         * Color of the line that connects the dumbbell point's values.
         * By default it is the series' color.
         *
         * @type        {string}
         * @since       8.0.0
         * @product     highcharts highstock
         * @apioption   series.dumbbell.data.connectorColor
         */
        /**
         * Pixel width of the line that connects the dumbbell point's values.
         *
         * @type        {number}
         * @since       8.0.0
         * @default     1
         * @product     highcharts highstock
         * @apioption   series.dumbbell.data.connectorWidth
         */
        /**
         * Color of the start markers in a dumbbell graph.
         *
         * @type        {Highcharts.ColorString|Highcharts.GradientColorObject|Highcharts.PatternObject}
         * @since       8.0.0
         * @default     #333333
         * @product     highcharts highstock
         * @apioption   series.dumbbell.data.lowColor
         */
        ''; // adds doclets above to transpiled file

    });
    _registerModule(_modules, 'masters/modules/dumbbell.src.js', [], function () {


    });
}));